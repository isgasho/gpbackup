package utils_test

import (
	"fmt"
	"os"
	"os/user"
	"sort"
	"strings"

	"github.com/greenplum-db/gpbackup/testutils"
	"github.com/greenplum-db/gpbackup/utils"
	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("utils/cluster tests", func() {
	masterSeg := utils.SegConfig{ContentID: -1, Hostname: "localhost", DataDir: "/data/gpseg-1"}
	localSegOne := utils.SegConfig{ContentID: 0, Hostname: "localhost", DataDir: "/data/gpseg0"}
	remoteSegOne := utils.SegConfig{ContentID: 1, Hostname: "remotehost1", DataDir: "/data/gpseg1"}
	remoteSegTwo := utils.SegConfig{ContentID: 2, Hostname: "remotehost2", DataDir: "/data/gpseg2"}
	var (
		testCluster  utils.Cluster
		testExecutor *testutils.TestExecutor
	)

	BeforeEach(func() {
		utils.System.CurrentUser = func() (*user.User, error) { return &user.User{Username: "testUser", HomeDir: "testDir"}, nil }
		utils.System.Hostname = func() (string, error) { return "testHost", nil }
		testExecutor = &testutils.TestExecutor{}
		testCluster = utils.NewCluster([]utils.SegConfig{masterSeg, localSegOne, remoteSegOne}, "", "20170101010101")
		testCluster.Executor = testExecutor
		testCluster.MetadataPipeFilePaths = []string{"globalPath", "predataPath", "postdataPath"}
		testCluster.TablePipeFilePaths = map[uint32]map[int]string{
			1234: map[int]string{0: "table1234", 1: "table1234"},
			2345: map[int]string{0: "table2345", 1: "table2345"},
		}
	})
	Describe("ConstructSSHCommand", func() {
		It("constructs an ssh command", func() {
			cmd := utils.ConstructSSHCommand("some-host", "ls")
			Expect(cmd).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@some-host", "ls"}))
		})
	})
	Describe("GenerateSSHCommandMap", func() {
		It("Returns a map of ssh commands for the master, including master", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg}, "", "20170101010101")
			commandMap := cluster.GenerateSSHCommandMap(true, func(_ int) string {
				return "ls"
			})
			Expect(len(commandMap)).To(Equal(1))
			Expect(commandMap[-1]).To(Equal([]string{"bash", "-c", "ls"}))
		})
		It("Returns a map of ssh commands for the master, excluding master", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg}, "", "20170101010101")
			commandMap := cluster.GenerateSSHCommandMap(false, func(_ int) string {
				return "ls"
			})
			Expect(len(commandMap)).To(Equal(0))
		})
		It("Returns a map of ssh commands for one segment, including master", func() {
			cluster := utils.NewCluster([]utils.SegConfig{remoteSegOne}, "", "20170101010101")
			commandMap := cluster.GenerateSSHCommandMap(true, func(_ int) string {
				return "ls"
			})
			Expect(len(commandMap)).To(Equal(1))
			Expect(commandMap[1]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost1", "ls"}))
		})
		It("Returns a map of ssh commands for one segment, excluding master", func() {
			cluster := utils.NewCluster([]utils.SegConfig{remoteSegOne}, "", "20170101010101")
			commandMap := cluster.GenerateSSHCommandMap(false, func(_ int) string {
				return "ls"
			})
			Expect(len(commandMap)).To(Equal(1))
			Expect(commandMap[1]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost1", "ls"}))
		})
		It("Returns a map of ssh commands for two segments on the same host, including master", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg, localSegOne}, "", "20170101010101")
			commandMap := cluster.GenerateSSHCommandMap(true, func(_ int) string {
				return "ls"
			})
			Expect(len(commandMap)).To(Equal(2))
			Expect(commandMap[-1]).To(Equal([]string{"bash", "-c", "ls"}))
			Expect(commandMap[0]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "ls"}))
		})
		It("Returns a map of ssh commands for two segments on the same host, excluding master", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg, localSegOne}, "", "20170101010101")
			commandMap := cluster.GenerateSSHCommandMap(false, func(_ int) string {
				return "ls"
			})
			Expect(len(commandMap)).To(Equal(1))
			Expect(commandMap[0]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "ls"}))
		})
		It("Returns a map of ssh commands for three segments on different hosts", func() {
			cluster := utils.NewCluster([]utils.SegConfig{localSegOne, remoteSegOne, remoteSegTwo}, "", "20170101010101")
			commandMap := cluster.GenerateSSHCommandMap(false, func(contentID int) string {
				return fmt.Sprintf("mkdir -p %s", cluster.GetDirForContent(contentID))
			})
			Expect(len(commandMap)).To(Equal(3))
			Expect(commandMap[0]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "mkdir -p /data/gpseg0/backups/20170101/20170101010101"}))
			Expect(commandMap[1]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost1", "mkdir -p /data/gpseg1/backups/20170101/20170101010101"}))
			Expect(commandMap[2]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost2", "mkdir -p /data/gpseg2/backups/20170101/20170101010101"}))
		})
	})
	Describe("GenerateSSHCommandMapForCluster", func() {
		It("includes master in the command map", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg, localSegOne}, "", "20170101010101")
			commandMap := cluster.GenerateSSHCommandMapForCluster(func(_ int) string {
				return "ls"
			})
			Expect(len(commandMap)).To(Equal(2))
			Expect(commandMap[-1]).To(Equal([]string{"bash", "-c", "ls"}))
			Expect(commandMap[0]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "ls"}))
		})
	})
	Describe("GenerateSSHCommandMapForSegments", func() {
		It("excludes master from the command map", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg, localSegOne}, "", "20170101010101")
			commandMap := cluster.GenerateSSHCommandMapForSegments(func(_ int) string {
				return "ls"
			})
			Expect(len(commandMap)).To(Equal(1))
			Expect(commandMap[0]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "ls"}))
		})
	})
	Describe("GenerateFileVerificationCommandMap", func() {
		It("creates a command map for segments only", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg, localSegOne, remoteSegOne}, "", "20170101010101")
			commandMap := cluster.GenerateFileVerificationCommandMap(13)

			Expect(len(commandMap)).To(Equal(2))
			Expect(commandMap[0]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "find /data/gpseg0/backups/20170101/20170101010101 -type f | wc -l | grep 13"}))
			Expect(commandMap[1]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost1", "find /data/gpseg1/backups/20170101/20170101010101 -type f | wc -l | grep 13"}))
		})
	})
	Describe("GenerateCreateAllTablePipesCommandMap", func() {
		It("returns a map of mkfifo commands for two segments on different hosts", func() {
			tables := []uint32{1, 2, 3}
			cluster := utils.NewCluster([]utils.SegConfig{localSegOne, remoteSegOne}, "", "20170101010101")
			commandMap := cluster.GenerateCreateAllTablePipesCommandMap(tables)
			Expect(len(commandMap)).To(Equal(2))
			Expect(commandMap[0]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "mkfifo /data/gpseg0/backups/20170101/20170101010101/gpbackup_0_20170101010101_1 /data/gpseg0/backups/20170101/20170101010101/gpbackup_0_20170101010101_2 /data/gpseg0/backups/20170101/20170101010101/gpbackup_0_20170101010101_3"}))
			Expect(commandMap[1]).To(Equal([]string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost1", "mkfifo /data/gpseg1/backups/20170101/20170101010101/gpbackup_1_20170101010101_1 /data/gpseg1/backups/20170101/20170101010101/gpbackup_1_20170101010101_2 /data/gpseg1/backups/20170101/20170101010101/gpbackup_1_20170101010101_3"}))
		})
		It("returns an empty map when there are no tables", func() {
			tables := []uint32{}
			cluster := utils.NewCluster([]utils.SegConfig{localSegOne, remoteSegOne}, "", "20170101010101")
			commandMap := cluster.GenerateCreateAllTablePipesCommandMap(tables)
			Expect(len(commandMap)).To(Equal(0))
		})
	})
	Describe("ExecuteLocalCommand", func() {
		BeforeEach(func() {
			os.MkdirAll("/tmp/backup_and_restore_test", 0777)
		})
		AfterEach(func() {
			os.RemoveAll("/tmp/backup_and_restore_test")
		})
		It("runs the specified command", func() {
			cluster := utils.Cluster{}
			commandStr := "touch /tmp/backup_and_restore_test/foo"
			cluster.Executor = &utils.GPDBExecutor{}
			cluster.ExecuteLocalCommand(commandStr)

			testutils.ExpectPathToExist("/tmp/backup_and_restore_test/foo")
		})
		It("returns any error generated by the specified command", func() {
			cluster := utils.Cluster{}
			commandStr := "some-non-existant-command /tmp/backup_and_restore_test/foo"
			cluster.Executor = &utils.GPDBExecutor{}
			err := cluster.ExecuteLocalCommand(commandStr)

			Expect(err.Error()).To(Equal("exit status 127"))
		})
	})
	Describe("ExecuteClusterCommand", func() {
		BeforeEach(func() {
			os.MkdirAll("/tmp/backup_and_restore_test", 0777)
		})
		AfterEach(func() {
			os.RemoveAll("/tmp/backup_and_restore_test")
		})
		It("runs commands specified by command map", func() {
			cluster := utils.Cluster{}
			commandMap := map[int][]string{
				-1: {"touch", "/tmp/backup_and_restore_test/foo"},
				0:  {"touch", "/tmp/backup_and_restore_test/baz"},
			}
			cluster.Executor = &utils.GPDBExecutor{}
			cluster.ExecuteClusterCommand(commandMap)

			testutils.ExpectPathToExist("/tmp/backup_and_restore_test/foo")
			testutils.ExpectPathToExist("/tmp/backup_and_restore_test/baz")
		})
		It("returns any errors generated by any of the commands", func() {
			cluster := utils.Cluster{}
			commandMap := map[int][]string{
				-1: {"touch", "/tmp/backup_and_restore_test/foo"},
				0:  {"some-non-existant-command"},
			}
			cluster.Executor = &utils.GPDBExecutor{}
			errMap := cluster.ExecuteClusterCommand(commandMap)

			testutils.ExpectPathToExist("/tmp/backup_and_restore_test/foo")
			Expect(len(errMap)).To(Equal(1))
			Expect(errMap[0].Error()).To(Equal("exec: \"some-non-existant-command\": executable file not found in $PATH"))
		})
	})
	Describe("LogFatalError", func() {
		It("logs an error for 1 segment", func() {
			cluster := utils.NewCluster(nil, "", "20170101010101")
			defer testutils.ShouldPanicWithMessage("Error occurred on 1 segment. See gbytes.Buffer for a complete list of segments with errors.")
			cluster.LogFatalError("Error occurred", 1)
		})
		It("logs an error for more than 1 segment", func() {
			cluster := utils.NewCluster(nil, "", "20170101010101")
			defer testutils.ShouldPanicWithMessage("Error occurred on 2 segments. See gbytes.Buffer for a complete list of segments with errors.")
			cluster.LogFatalError("Error occurred", 2)
		})
	})
	Describe("cluster setup and accessor functions", func() {
		masterSeg := utils.SegConfig{ContentID: -1, Hostname: "localhost", DataDir: "/data/gpseg-1"}
		localSegOne := utils.SegConfig{ContentID: 0, Hostname: "localhost", DataDir: "/data/gpseg0"}
		localSegTwo := utils.SegConfig{ContentID: 1, Hostname: "localhost", DataDir: "/data/gpseg1"}
		remoteSegTwo := utils.SegConfig{ContentID: 1, Hostname: "remotehost", DataDir: "/data/gpseg1"}
		It("returns content dir for a single-host, single-segment nodes", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg, localSegOne}, "", "20170101010101")
			Expect(len(cluster.GetContentList())).To(Equal(2))
			Expect(cluster.GetDirForContent(-1)).To(Equal("/data/gpseg-1/backups/20170101/20170101010101"))
			Expect(cluster.GetHostForContent(-1)).To(Equal("localhost"))
			Expect(cluster.GetDirForContent(0)).To(Equal("/data/gpseg0/backups/20170101/20170101010101"))
			Expect(cluster.GetHostForContent(0)).To(Equal("localhost"))
		})
		It("sets up the configuration for a single-host, multi-segment cluster", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg, localSegOne, localSegTwo}, "", "20170101010101")
			Expect(len(cluster.GetContentList())).To(Equal(3))
			Expect(cluster.GetDirForContent(-1)).To(Equal("/data/gpseg-1/backups/20170101/20170101010101"))
			Expect(cluster.GetHostForContent(-1)).To(Equal("localhost"))
			Expect(cluster.GetDirForContent(0)).To(Equal("/data/gpseg0/backups/20170101/20170101010101"))
			Expect(cluster.GetHostForContent(0)).To(Equal("localhost"))
			Expect(cluster.GetDirForContent(1)).To(Equal("/data/gpseg1/backups/20170101/20170101010101"))
			Expect(cluster.GetHostForContent(1)).To(Equal("localhost"))
		})
		It("sets up the configuration for a multi-host, multi-segment cluster", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg, localSegOne, remoteSegTwo}, "", "20170101010101")
			Expect(len(cluster.GetContentList())).To(Equal(3))
			Expect(cluster.GetDirForContent(-1)).To(Equal("/data/gpseg-1/backups/20170101/20170101010101"))
			Expect(cluster.GetHostForContent(-1)).To(Equal("localhost"))
			Expect(cluster.GetDirForContent(0)).To(Equal("/data/gpseg0/backups/20170101/20170101010101"))
			Expect(cluster.GetHostForContent(0)).To(Equal("localhost"))
			Expect(cluster.GetDirForContent(1)).To(Equal("/data/gpseg1/backups/20170101/20170101010101"))
			Expect(cluster.GetHostForContent(1)).To(Equal("remotehost"))
		})
		It("returns the content directory based on the user specified path", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg}, "/foo/bar", "20170101010101")
			Expect(cluster.GetDirForContent(-1)).To(Equal("/foo/bar/gpseg-1/backups/20170101/20170101010101"))
		})
	})
	Describe("GetTableBackupFilePathForCopyCommand()", func() {
		It("returns table file path for copy command", func() {
			cluster := utils.NewCluster(nil, "", "20170101010101")
			Expect(cluster.GetTableBackupFilePathForCopyCommand(1234)).To(Equal("<SEG_DATA_DIR>/backups/20170101/20170101010101/gpbackup_<SEGID>_20170101010101_1234"))
		})
		It("returns table file path for copy command based on user specifed path", func() {
			cluster := utils.NewCluster(nil, "/foo/bar", "20170101010101")
			Expect(cluster.GetTableBackupFilePathForCopyCommand(1234)).To(Equal("/foo/bar/gpseg<SEGID>/backups/20170101/20170101010101/gpbackup_<SEGID>_20170101010101_1234"))
		})
	})
	Describe("GetTableMapFilePath()", func() {
		It("returns table map file path", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg}, "", "20170101010101")
			Expect(cluster.GetTableMapFilePath()).To(Equal("/data/gpseg-1/backups/20170101/20170101010101/gpbackup_20170101010101_table_map"))
		})
		It("returns table map file path based on user specified path", func() {
			cluster := utils.NewCluster(nil, "/foo/bar", "20170101010101")
			Expect(cluster.GetTableMapFilePath()).To(Equal("/foo/bar/gpseg-1/backups/20170101/20170101010101/gpbackup_20170101010101_table_map"))
		})
	})
	Describe("GetReportFilePath", func() {
		It("returns report file path", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg}, "", "20170101010101")
			Expect(cluster.GetReportFilePath()).To(Equal("/data/gpseg-1/backups/20170101/20170101010101/gpbackup_20170101010101_report"))
		})
		It("returns report file path based on user specified path", func() {
			cluster := utils.NewCluster(nil, "/foo/bar", "20170101010101")
			Expect(cluster.GetReportFilePath()).To(Equal("/foo/bar/gpseg-1/backups/20170101/20170101010101/gpbackup_20170101010101_report"))
		})
	})
	Describe("GetTableBackupFilePath", func() {
		It("returns table file path", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg}, "", "20170101010101")
			Expect(cluster.GetTableBackupFilePath(-1, 1234)).To(Equal("/data/gpseg-1/backups/20170101/20170101010101/gpbackup_-1_20170101010101_1234"))
		})
		It("returns table file path based on user specified path", func() {
			cluster := utils.NewCluster([]utils.SegConfig{masterSeg}, "/foo/bar", "20170101010101")
			Expect(cluster.GetTableBackupFilePath(-1, 1234)).To(Equal("/foo/bar/gpseg-1/backups/20170101/20170101010101/gpbackup_-1_20170101010101_1234"))
		})
	})
	Describe("ExecuteClusterCommandForEachTableFile", func() {
		BeforeEach(func() {
			testCluster.TablePipeFilePaths = make(map[uint32]map[int]string, 0)
			testCluster.TablePipeFilePaths[1234] = map[int]string{
				0: "/data/gpseg0/backups/20170101/20170101010101/gpbackup_0_20170101010101_1234",
				1: "/data/gpseg1/backups/20170101/20170101010101/gpbackup_1_20170101010101_1234",
			}
			testCluster.TablePipeFilePaths[2345] = map[int]string{
				0: "/data/gpseg0/backups/20170101/20170101010101/gpbackup_0_20170101010101_2345",
				1: "/data/gpseg1/backups/20170101/20170101010101/gpbackup_1_20170101010101_2345",
			}
		})
		It("executes a command for one table on all segments", func() {
			delete(testCluster.TablePipeFilePaths, 1234)
			expectedOid := uint32(0)
			var expectedMap map[int]error
			errOid, errMap := testCluster.ExecuteClusterCommandForEachTableFile("cat <TABLE> > <TABLE>.out")
			Expect((*testExecutor).NumExecutions).To(Equal(1))
			Expect(errOid).To(Equal(expectedOid))
			Expect(errMap).To(Equal(expectedMap))
		})
		It("executes a command for all tables on all segments", func() {
			expectedOid := uint32(0)
			var expectedMap map[int]error
			errOid, errMap := testCluster.ExecuteClusterCommandForEachTableFile("cat <TABLE> > <TABLE>.out")
			Expect((*testExecutor).NumExecutions).To(Equal(2))
			Expect(errOid).To(Equal(expectedOid))
			Expect(errMap).To(Equal(expectedMap))
		})
		It("fails if ExecuteClusterCommand errors out", func() {
			expectedOid := uint32(0)
			expectedMap := map[int]error{
				0: errors.Errorf("exit status 1"),
				1: errors.Errorf("exit status 1"),
			}
			(*testExecutor).ClusterError = expectedMap
			testCluster.Executor = testExecutor
			errOid, errMap := testCluster.ExecuteClusterCommandForEachTableFile("cat <TABLE> > <TABLE>.out")
			Expect((*testExecutor).NumExecutions).To(Equal(2))
			if strings.Contains((*testExecutor).ClusterCommands[0][0][4], "1234") {
				expectedOid = uint32(2345)
			} else {
				expectedOid = uint32(1234)
			}
			Expect(errOid).To(Equal(expectedOid))
			Expect(errMap).To(Equal(expectedMap))
		})
	})
	Describe("DeleteAllMetadataPipes", func() {
		It("successfully deletes all metadata pipes", func() {
			testCluster.DeleteAllMetadataPipes()
			Expect((*testExecutor).NumExecutions).To(Equal(3))
		})
		It("panics if it cannot delete all metadata pipes", func() {
			testExecutor.LocalError = errors.Errorf("exit status 1")
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Unable to delete globalPath pipe")
			testCluster.DeleteAllMetadataPipes()
		})
	})
	Describe("DeleteAllTablePipes", func() {
		It("successfully deletes one table pipe", func() {
			delete(testCluster.TablePipeFilePaths, 1234)
			testCluster.DeleteAllTablePipes()
			Expect((*testExecutor).NumExecutions).To(Equal(1))
		})
		It("successfully deletes all table pipes", func() {
			testCluster.DeleteAllTablePipes()
			Expect((*testExecutor).NumExecutions).To(Equal(2))
		})
		It("panics if it cannot delete all table pipes", func() {
			testExecutor.ClusterError = map[int]error{
				0: errors.Errorf("exit status 1"),
				1: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Unable to delete pipes on 2 segments")
			testCluster.DeleteAllTablePipes()
		})
		It("panics if it cannot delete some table pipes", func() {
			testExecutor.ClusterError = map[int]error{
				0: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Unable to delete pipes on 1 segment")
			testCluster.DeleteAllTablePipes()
		})
	})
	Describe("VerifyBackupFileCountOnSegments", func() {
		It("successfully verifies all backup file counts", func() {
			testCluster.VerifyBackupFileCountOnSegments(2)
			Expect((*testExecutor).NumExecutions).To(Equal(1))
		})
		It("panics if it cannot verify all backup file counts", func() {
			testExecutor.ClusterError = map[int]error{
				0: errors.Errorf("exit status 1"),
				1: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Backup files missing on 2 segments")
			testCluster.VerifyBackupFileCountOnSegments(2)
		})
		It("panics if it cannot verify some backup file counts", func() {
			testExecutor.ClusterError = map[int]error{
				1: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Backup files missing on 1 segment")
			testCluster.VerifyBackupFileCountOnSegments(2)
		})
	})
	Describe("VerifyBackupDirectoriesExistOnAllHosts", func() {
		It("successfully verifies all directories", func() {
			testCluster.VerifyBackupDirectoriesExistOnAllHosts()
			Expect((*testExecutor).NumExecutions).To(Equal(1))
		})
		It("panics if it cannot verify all directories", func() {
			testExecutor.ClusterError = map[int]error{
				0: errors.Errorf("exit status 1"),
				1: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Directories missing or inaccessible on 2 segments")
			testCluster.VerifyBackupDirectoriesExistOnAllHosts()
		})
		It("panics if it cannot verify some directories", func() {
			testExecutor.ClusterError = map[int]error{
				1: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Directories missing or inaccessible on 1 segment")
			testCluster.VerifyBackupDirectoriesExistOnAllHosts()
		})
	})
	Describe("CreateBackupDirectoriesOnAllHosts", func() {
		It("successfully creates all directories", func() {
			testCluster.CreateBackupDirectoriesOnAllHosts()
			Expect((*testExecutor).NumExecutions).To(Equal(1))
		})
		It("panics if it cannot create all directories", func() {
			testExecutor.ClusterError = map[int]error{
				0: errors.Errorf("exit status 1"),
				1: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Unable to create directories on 2 segments")
			testCluster.CreateBackupDirectoriesOnAllHosts()
		})
		It("panics if it cannot create some directories", func() {
			testExecutor.ClusterError = map[int]error{
				1: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Unable to create directories on 1 segment")
			testCluster.CreateBackupDirectoriesOnAllHosts()
		})
	})
	Describe("CreateAllMetadataPipes", func() {
		It("successfully creates all metadata pipes", func() {
			testCluster.CreateAllMetadataPipes()
			Expect((*testExecutor).NumExecutions).To(Equal(1))
		})
		It("panics if it cannot create some or all metadata pipes", func() {
			testExecutor.LocalError = errors.Errorf("exit status 1")
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Unable to create metadata file pipes: exit status 1")
			testCluster.CreateAllMetadataPipes()
		})
	})
	Describe("CreateAllTablePipes", func() {
		var tableOids []uint32
		BeforeEach(func() {
			tableOids = []uint32{1234, 2345}
		})
		It("successfully creates one table pipe", func() {
			tableOids = []uint32{2345}
			delete(testCluster.TablePipeFilePaths, 1234)
			testCluster.CreateAllTablePipes(tableOids)
			Expect((*testExecutor).NumExecutions).To(Equal(1))
		})
		It("successfully creates all table pipes", func() {
			testCluster.CreateAllTablePipes(tableOids)
			Expect((*testExecutor).NumExecutions).To(Equal(1))
		})
		It("panics if it cannot create all table pipes", func() {
			testExecutor.ClusterError = map[int]error{
				0: errors.Errorf("exit status 1"),
				1: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Unable to create data file pipes on 2 segments")
			testCluster.CreateAllTablePipes(tableOids)
		})
		It("panics if it cannot create some table pipes", func() {
			testExecutor.ClusterError = map[int]error{
				0: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Unable to create data file pipes on 1 segment")
			testCluster.CreateAllTablePipes(tableOids)
		})
	})
	Describe("ReadFromAllMetadataPipes", func() {
		It("successfully reads when using compression and a plugin", func() {
			testCluster.ReadFromAllMetadataPipes(true, true)
			Expect((*testExecutor).NumExecutions).To(Equal(3))
			expectedCommands := []string{
				`bash -c "cat globalPath | gzip -c > globalPath.gz.out"`,
				`bash -c "cat postdataPath | gzip -c > postdataPath.gz.out"`,
				`bash -c "cat predataPath | gzip -c > predataPath.gz.out"`,
			}
			executedCommands := (*testExecutor).LocalCommands
			Expect(len(executedCommands)).To(Equal(3))
			sort.Strings(executedCommands)
			Expect(executedCommands).To(Equal(expectedCommands))
		})
		It("successfully reads when using compression without a plugin", func() {
			testCluster.ReadFromAllMetadataPipes(true, false)
			Expect((*testExecutor).NumExecutions).To(Equal(3))
			expectedCommands := []string{
				`bash -c "cat globalPath | gzip -c > globalPath.gz"`,
				`bash -c "cat postdataPath | gzip -c > postdataPath.gz"`,
				`bash -c "cat predataPath | gzip -c > predataPath.gz"`,
			}
			executedCommands := (*testExecutor).LocalCommands
			Expect(len(executedCommands)).To(Equal(3))
			sort.Strings(executedCommands)
			Expect(executedCommands).To(Equal(expectedCommands))
		})
		It("successfully reads when using a plugin without compression", func() {
			testCluster.ReadFromAllMetadataPipes(false, true)
			Expect((*testExecutor).NumExecutions).To(Equal(3))
			expectedCommands := []string{
				`bash -c "cat globalPath > globalPath.out"`,
				`bash -c "cat postdataPath > postdataPath.out"`,
				`bash -c "cat predataPath > predataPath.out"`,
			}
			executedCommands := (*testExecutor).LocalCommands
			Expect(len(executedCommands)).To(Equal(3))
			sort.Strings(executedCommands)
			Expect(executedCommands).To(Equal(expectedCommands))
		})
		It("panics if it cannot read from some or all metadata pipes", func() {
			testExecutor.LocalError = errors.Errorf("exit status 1")
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Unable to read from globalPath pipe: exit status 1")
			testCluster.ReadFromAllMetadataPipes(true, true)
		})
	})
	Describe("ReadFromAllTablePipes", func() {
		It("successfully reads when using compression and a plugin", func() {
			testCluster.ReadFromAllTablePipes(true, true)
			Expect((*testExecutor).NumExecutions).To(Equal(2))
			expectedClusterCommands := []map[int][]string{
				{
					0: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "cat table1234 | gzip -c > table1234.gz.out"},
					1: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost1", "cat table1234 | gzip -c > table1234.gz.out"},
				},
				{
					0: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "cat table2345 | gzip -c > table2345.gz.out"},
					1: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost1", "cat table2345 | gzip -c > table2345.gz.out"},
				},
			}
			executedClusterCommands := (*testExecutor).ClusterCommands
			Expect(len(executedClusterCommands)).To(Equal(2))
			if strings.Contains(executedClusterCommands[0][0][4], "2345") { // Account for commands being executed out of order
				executedClusterCommands = []map[int][]string{executedClusterCommands[1], executedClusterCommands[0]}
			}
			Expect(executedClusterCommands).To(Equal(expectedClusterCommands))
		})
		It("successfully reads when using compression without a plugin", func() {
			testCluster.ReadFromAllTablePipes(true, false)
			Expect((*testExecutor).NumExecutions).To(Equal(2))
			expectedClusterCommands := []map[int][]string{
				{
					0: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "cat table1234 | gzip -c > table1234.gz"},
					1: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost1", "cat table1234 | gzip -c > table1234.gz"},
				},
				{
					0: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "cat table2345 | gzip -c > table2345.gz"},
					1: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost1", "cat table2345 | gzip -c > table2345.gz"},
				},
			}
			executedClusterCommands := (*testExecutor).ClusterCommands
			Expect(len(executedClusterCommands)).To(Equal(2))
			if strings.Contains(executedClusterCommands[0][0][4], "2345") {
				executedClusterCommands = []map[int][]string{executedClusterCommands[1], executedClusterCommands[0]}
			}
			Expect(executedClusterCommands).To(Equal(expectedClusterCommands))
		})
		It("successfully reads when using a plugin without compression", func() {
			testCluster.ReadFromAllTablePipes(false, true)
			Expect((*testExecutor).NumExecutions).To(Equal(2))
			expectedClusterCommands := []map[int][]string{
				{
					0: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "cat table1234 > table1234.out"},
					1: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost1", "cat table1234 > table1234.out"},
				},
				{
					0: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@localhost", "cat table2345 > table2345.out"},
					1: []string{"ssh", "-o", "StrictHostKeyChecking=no", "testUser@remotehost1", "cat table2345 > table2345.out"},
				},
			}
			executedClusterCommands := (*testExecutor).ClusterCommands
			Expect(len(executedClusterCommands)).To(Equal(2))
			if strings.Contains(executedClusterCommands[0][0][4], "2345") {
				executedClusterCommands = []map[int][]string{executedClusterCommands[1], executedClusterCommands[0]}
			}
			Expect(executedClusterCommands).To(Equal(expectedClusterCommands))
		})
		It("panics if it cannot read from all table pipes", func() {
			testExecutor.ClusterError = map[int]error{
				0: errors.Errorf("exit status 1"),
				1: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Unable to read data from pipes on 2 segments")
			testCluster.ReadFromAllTablePipes(true, true)
		})
		It("panics if it cannot read from some table pipes", func() {
			testExecutor.ClusterError = map[int]error{
				0: errors.Errorf("exit status 1"),
			}
			testCluster.Executor = testExecutor
			defer testutils.ShouldPanicWithMessage("Unable to read data from pipes on 1 segment")
			testCluster.ReadFromAllTablePipes(true, true)
		})
	})
})
