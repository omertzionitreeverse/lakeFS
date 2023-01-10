package esti

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/treeverse/lakefs/pkg/api"
	"github.com/treeverse/lakefs/pkg/block"
)

type GCMode int

const (
	fullGCMode GCMode = iota
	markOnlyMode
	sweepOnlyMode
)

const (
	committedGCRepoName = "gc"
)

var (
	currentEpochInSeconds = time.Now().Unix()
	dayInSeconds          = int64(100000) // rounded up from 86400
)

type testCase struct {
	id           string
	policy       api.GarbageCollectionRules
	branches     []branchProperty
	fileDeleted  bool
	description  string
	directUpload bool
	testMode     GCMode
}

type branchProperty struct {
	name                string
	deleteCommitDaysAgo int64
}

var testCases = []testCase{
	{
		id:     "1",
		policy: api.GarbageCollectionRules{Branches: []api.GarbageCollectionRule{}, DefaultRetentionDays: 1},
		branches: []branchProperty{
			{name: "a1", deleteCommitDaysAgo: 2}, {name: "b1", deleteCommitDaysAgo: 2},
		},
		fileDeleted:  true,
		description:  "The file is deleted according to the default retention policy",
		directUpload: false,
	},
	{
		id:     "2",
		policy: api.GarbageCollectionRules{Branches: []api.GarbageCollectionRule{{BranchId: "a2", RetentionDays: 1}, {BranchId: "b2", RetentionDays: 3}}, DefaultRetentionDays: 5},
		branches: []branchProperty{
			{name: "a2", deleteCommitDaysAgo: 4}, {name: "b2", deleteCommitDaysAgo: 4},
		},
		fileDeleted:  true,
		description:  "The file is deleted according to branches' retention policies",
		directUpload: false,
	},
	{
		id:     "3",
		policy: api.GarbageCollectionRules{Branches: []api.GarbageCollectionRule{{BranchId: "a3", RetentionDays: 1}, {BranchId: "b3", RetentionDays: 3}}, DefaultRetentionDays: 5},
		branches: []branchProperty{
			{name: "a3", deleteCommitDaysAgo: 4}, {name: "b3", deleteCommitDaysAgo: 2},
		},
		fileDeleted:  false,
		description:  "The file is not deleted because of the retention policy of the second branch",
		directUpload: false,
	},
	{
		id:     "4",
		policy: api.GarbageCollectionRules{Branches: []api.GarbageCollectionRule{}, DefaultRetentionDays: 5},
		branches: []branchProperty{
			{name: "a4", deleteCommitDaysAgo: 4}, {name: "b4", deleteCommitDaysAgo: 2},
		},
		fileDeleted:  false,
		description:  "The file isn't deleted according to default retention policy",
		directUpload: false,
	},
	{
		id:     "5",
		policy: api.GarbageCollectionRules{Branches: []api.GarbageCollectionRule{{BranchId: "a5", RetentionDays: 1}, {BranchId: "b5", RetentionDays: 3}}, DefaultRetentionDays: 5},
		branches: []branchProperty{
			{name: "a5", deleteCommitDaysAgo: 1},
		},
		fileDeleted:  false,
		description:  "The file is not deleted as it still exists in the second branch",
		directUpload: false,
	},
	{
		id:     "6",
		policy: api.GarbageCollectionRules{Branches: []api.GarbageCollectionRule{{BranchId: "a6", RetentionDays: 1}}, DefaultRetentionDays: 5},
		branches: []branchProperty{
			{name: "a6", deleteCommitDaysAgo: 1}, {name: "b6", deleteCommitDaysAgo: 4},
		},
		fileDeleted:  false,
		description:  "The file is not deleted because default retention time keeps it for the second branch",
		directUpload: false,
	},
	{
		id:     "7",
		policy: api.GarbageCollectionRules{Branches: []api.GarbageCollectionRule{{BranchId: "a7", RetentionDays: 1}}, DefaultRetentionDays: 5},
		branches: []branchProperty{
			{name: "a7", deleteCommitDaysAgo: 1}, {name: "b7", deleteCommitDaysAgo: 5},
		},
		fileDeleted:  true,
		description:  "The file is deleted as the retention policy for the branch permits the deletion from the branch, and the default retention policy permits deletion for the second branch",
		directUpload: false,
	},
	{
		id:     "8",
		policy: api.GarbageCollectionRules{Branches: []api.GarbageCollectionRule{{BranchId: "a8", RetentionDays: 3}}, DefaultRetentionDays: 1},
		branches: []branchProperty{
			{name: "a8", deleteCommitDaysAgo: 2}, {name: "b8", deleteCommitDaysAgo: -1},
		},
		fileDeleted:  false,
		description:  "The file (direct upload) is not deleted as the branch retention policy overrules the default retention policy",
		directUpload: true,
	},
	{
		id:     "9",
		policy: api.GarbageCollectionRules{Branches: []api.GarbageCollectionRule{}, DefaultRetentionDays: 1},
		branches: []branchProperty{
			{name: "a9", deleteCommitDaysAgo: -1}, {name: "b9", deleteCommitDaysAgo: -1},
		},
		fileDeleted:  true,
		description:  "The file (direct upload) is deleted because it's in a dangling commit and the default retention policy has passed",
		directUpload: true,
	},
	{
		id:     "10",
		policy: api.GarbageCollectionRules{Branches: []api.GarbageCollectionRule{}, DefaultRetentionDays: 1},
		branches: []branchProperty{
			{name: "a10", deleteCommitDaysAgo: -1}, {name: "b10", deleteCommitDaysAgo: -1},
		},
		fileDeleted:  false,
		description:  "The file (direct upload) is only marked for deletion (not actually deleted)",
		directUpload: true,
		testMode:     markOnlyMode,
	},
	{
		id:     "11",
		policy: api.GarbageCollectionRules{Branches: []api.GarbageCollectionRule{}, DefaultRetentionDays: 1},
		branches: []branchProperty{
			{name: "a11", deleteCommitDaysAgo: -1}, {name: "b11", deleteCommitDaysAgo: -1},
		},
		fileDeleted:  true,
		description:  "The file (direct upload) is swept after it's first marked",
		directUpload: true,
		testMode:     sweepOnlyMode,
	},
}

func TestCommittedGC(t *testing.T) {
	SkipTestIfAskedTo(t)
	blockstoreType := viper.GetViper().GetString("blockstore_type")
	// TODO lynn: change this for test also on Azure
	if blockstoreType != block.BlockstoreTypeS3 {
		t.Skip("Running on S3 only")
	}
	ctx := context.Background()
	for _, tst := range testCases {
		tst := tst // re-define tst to be in the scope of the closure. See: https://gist.github.com/posener/92a55c4cd441fc5e5e85f27bca008721
		t.Run(fmt.Sprintf("Test case %s", tst.id), func(t *testing.T) {
			fileExistingRef := prepareForGC(t, ctx, &tst, blockstoreType)
			t.Parallel()
			t.Logf("fileExistingRef %s", fileExistingRef)
			repo := committedGCRepoName + tst.id
			if tst.testMode == sweepOnlyMode || tst.testMode == markOnlyMode {
				runGC(t, repo, "--conf", "spark.hadoop.lakefs.gc.do_sweep=false",
					"--conf", fmt.Sprintf("spark.hadoop.lakefs.gc.mark_id=marker%s", tst.id))
			}
			if tst.testMode == sweepOnlyMode {
				runGC(t, repo, "--conf", "spark.hadoop.lakefs.gc.do_mark=false",
					"--conf", fmt.Sprintf("spark.hadoop.lakefs.gc.mark_id=marker%s", tst.id))
			}
			if tst.testMode == fullGCMode {
				runGC(t, repo)
			}
			validateGCJob(t, ctx, &tst, fileExistingRef)
		})
	}

}

func prepareForGC(t *testing.T, ctx context.Context, testCase *testCase, blockstoreType string) string {
	repo := createRepositoryByName(ctx, t, committedGCRepoName+testCase.id)

	// upload 3 files not to be deleted and commit
	_, _ = uploadFileRandomData(ctx, t, repo, mainBranch, "not_deleted_file1", false)
	_, _ = uploadFileRandomData(ctx, t, repo, mainBranch, "not_deleted_file2", false)
	direct := blockstoreType == block.BlockstoreTypeS3
	_, _ = uploadFileRandomData(ctx, t, repo, mainBranch, "not_deleted_file3", direct)

	commitTime := int64(0)
	_, err := client.CommitWithResponse(ctx, repo, mainBranch, &api.CommitParams{}, api.CommitJSONRequestBody{Message: "add three files not to be deleted", Date: &commitTime})
	if err != nil {
		t.Fatalf("Commit some data %s", err)
	}

	newBranch := "a" + testCase.id
	_, err = client.CreateBranchWithResponse(ctx, repo, api.CreateBranchJSONRequestBody{Name: newBranch, Source: mainBranch})
	if err != nil {
		t.Fatalf("Create new branch %s", err)
	}

	direct = testCase.directUpload && blockstoreType == block.BlockstoreTypeS3
	_, _ = uploadFileRandomData(ctx, t, repo, newBranch, "file"+testCase.id, direct)
	commitTime = int64(10)

	// get commit id after commit for validation step in the tests
	commitRes, err := client.CommitWithResponse(ctx, repo, newBranch, &api.CommitParams{}, api.CommitJSONRequestBody{Message: "Uploaded file" + testCase.id, Date: &commitTime})
	if err != nil || commitRes.StatusCode() != 201 {
		t.Fatalf("Commit some data %s", err)
	}
	commit := commitRes.JSON201
	commitId := commit.Id

	_, err = client.CreateBranchWithResponse(ctx, repo, api.CreateBranchJSONRequestBody{Name: "b" + testCase.id, Source: newBranch})
	if err != nil {
		t.Fatalf("Create new branch %s", err)
	}

	_, err = client.SetGarbageCollectionRulesWithResponse(ctx, repo, api.SetGarbageCollectionRulesJSONRequestBody{Branches: testCase.policy.Branches, DefaultRetentionDays: testCase.policy.DefaultRetentionDays})
	if err != nil {
		t.Fatalf("Set GC rules %s", err)
	}

	for _, branch := range testCase.branches {
		if branch.deleteCommitDaysAgo > -1 {
			_, err = client.DeleteObjectWithResponse(ctx, repo, branch.name, &api.DeleteObjectParams{Path: "file" + testCase.id})
			if err != nil {
				t.Fatalf("DeleteObject %s", err)
			}
			epochCommitDateInSeconds := currentEpochInSeconds - (dayInSeconds * branch.deleteCommitDaysAgo)
			_, err = client.CommitWithResponse(ctx, repo, branch.name, &api.CommitParams{}, api.CommitJSONRequestBody{Message: "Deleted file" + testCase.id, Date: &epochCommitDateInSeconds})
			if err != nil {
				t.Fatalf("Commit some data %s", err)
			}
			_, _ = uploadFileRandomData(ctx, t, repo, branch.name, "file"+testCase.id+"not_deleted", false)
			// This is for the previous commit to be the HEAD of the branch outside the retention time (according to GC https://github.com/treeverse/lakeFS/issues/1932)
			_, err = client.CommitWithResponse(ctx, repo, branch.name, &api.CommitParams{}, api.CommitJSONRequestBody{Message: "not deleted file commit: " + testCase.id, Date: &epochCommitDateInSeconds})
			if err != nil {
				t.Fatalf("Commit some data %s", err)
			}
		} else {
			_, err = client.DeleteBranchWithResponse(ctx, repo, branch.name)
			if err != nil {
				t.Fatalf("Delete brach %s", err)
			}
		}
	}
	return commitId
}

func getSparkSubmitArgs() []string {
	return []string{
		"--master", "spark://localhost:7077",
		"--conf", "spark.driver.extraJavaOptions=-Divy.cache.dir=/tmp -Divy.home=/tmp",
		"--conf", "spark.hadoop.lakefs.api.url=http://lakefs:8000/api/v1",
		"--conf", "spark.hadoop.lakefs.api.access_key=AKIAIOSFDNN7EXAMPLEQ",
		"--conf", "spark.hadoop.lakefs.api.secret_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		"--class", "io.treeverse.clients.GarbageCollector",
		"--packages", "org.apache.hadoop:hadoop-aws:3.2.4",
	}
}

func getDockerArgs(workingDirectory string) []string {
	return []string{"run", "--network", "host", "--add-host", "lakefs:127.0.0.1",
		"-v", fmt.Sprintf("%s/ivy:/opt/bitnami/spark/.ivy2", workingDirectory),
		"-v", fmt.Sprintf("%s:/opt/metaclient/client.jar", metaclientJarPath),
		"--rm",
		"-e", "AWS_ACCESS_KEY_ID",
		"-e", "AWS_SECRET_ACCESS_KEY",
	}
}

func runGC(t *testing.T, repo string, extraSparkArgs ...string) {
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal("Failed getting working directory: ", err)
	}
	workingDirectory = strings.TrimSuffix(workingDirectory, "/")
	dockerArgs := getDockerArgs(workingDirectory)
	dockerArgs = append(dockerArgs, "docker.io/bitnami/spark:3.2", "spark-submit")
	sparkSubmitArgs := getSparkSubmitArgs()
	sparkSubmitArgs = append(sparkSubmitArgs, extraSparkArgs...)
	args := append(dockerArgs, sparkSubmitArgs...)
	args = append(args, "/opt/metaclient/client.jar", repo, "us-east-1")
	cmd := exec.Command("docker", args...)
	err = runCommand(fmt.Sprintf("gc-%s", repo), cmd)
	if err != nil {
		t.Fatal("Running GC: ", err)
	}
}

func validateGCJob(t *testing.T, ctx context.Context, testCase *testCase, existingRef string) {
	repo := committedGCRepoName + testCase.id

	res, _ := client.GetObjectWithResponse(ctx, repo, existingRef, &api.GetObjectParams{Path: "file" + testCase.id})
	fileExists := res.StatusCode() == 200

	if fileExists && testCase.fileDeleted {
		t.Errorf("Expected the file to be removed by the garbage collector but it has remained in the repository. Test case '%s'. Test description '%s'", testCase.id, testCase.description)
	} else if !fileExists && !testCase.fileDeleted {
		t.Errorf("Expected the file to remain in the repository but it was removed by the garbage collector. Test case '%s'. Test description '%s'", testCase.id, testCase.description)
	}
	locations := []string{"not_deleted_file1", "not_deleted_file2", "not_deleted_file3"}
	for _, location := range locations {
		res, _ = client.GetObjectWithResponse(ctx, repo, "main", &api.GetObjectParams{Path: location})
		if res.StatusCode() != 200 {
			t.Errorf("expected '%s' to exist. Test case '%s', Test description '%s'", location, testCase.id, testCase.description)
		}
	}
}

// runCommand runs the given command. It redirects the output of the command:
// 1. stdout is redirected to the logger.
// 2. stderr is simply printed out.
func runCommand(cmdName string, cmd *exec.Cmd) error {
	handlePipe := func(pipe io.ReadCloser, log func(format string, args ...interface{})) {
		reader := bufio.NewReader(pipe)
		go func() {
			defer func() {
				_ = pipe.Close()
			}()
			for {
				str, err := reader.ReadString('\n')
				if err != nil {
					break
				}
				log(strings.TrimSuffix(str, "\n"))
			}
		}()
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout from command: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr from command: %w", err)
	}
	handlePipe(stdoutPipe, logger.WithField("source", cmdName).Infof)
	handlePipe(stderrPipe, func(format string, args ...interface{}) {
		println(format, args)
	})
	err = cmd.Start()
	if err != nil {
		return err
	}
	return cmd.Wait()
}