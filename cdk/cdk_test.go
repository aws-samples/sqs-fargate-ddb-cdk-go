package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
)

func TestTestCdkGoStack(t *testing.T) {
	// GIVEN
	app := awscdk.NewApp(nil)

	tempFile, deleteFunc := createTempFile(t, `test credentials`)
	defer deleteFunc()

	config := &ProjectConfig{
		ProjectBaseDirectory: projectDirectory(t),
		NatsCredentialsFile:  tempFile,
		TableName:            "my-table",
		ClusterName:          "my-cluster",
	}

	// WHEN
	stack := NewTestCdkGoStack(app, "MyStack", config, nil)

	// THEN
	template := assertions.Template_FromStack(stack, nil)
	prettyPrint(t, template)

	// To construct the following list:
	// - Saved the prettyprint above into a file template.json
	// - Ran `jq '.Resources | map(.Type) | group_by(.) | map({(.[0]): length}) | add' template.json`
	template.ResourceCountIs(jsii.String("AWS::DynamoDB::GlobalTable"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::EC2::EIP"), jsii.Number(2))
	template.ResourceCountIs(jsii.String("AWS::EC2::InternetGateway"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::EC2::NatGateway"), jsii.Number(2))
	template.ResourceCountIs(jsii.String("AWS::EC2::Route"), jsii.Number(4))
	template.ResourceCountIs(jsii.String("AWS::EC2::RouteTable"), jsii.Number(4))
	template.ResourceCountIs(jsii.String("AWS::EC2::SecurityGroup"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::EC2::Subnet"), jsii.Number(4))
	template.ResourceCountIs(jsii.String("AWS::EC2::SubnetRouteTableAssociation"), jsii.Number(4))
	template.ResourceCountIs(jsii.String("AWS::EC2::VPC"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::EC2::VPCGatewayAttachment"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::ECS::Cluster"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::ECS::Service"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::ECS::TaskDefinition"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::IAM::Policy"), jsii.Number(2))
	template.ResourceCountIs(jsii.String("AWS::IAM::Role"), jsii.Number(2))
	template.ResourceCountIs(jsii.String("AWS::Logs::LogGroup"), jsii.Number(1))
}

func prettyPrint(t *testing.T, template assertions.Template) {
	// Marshal the JSON data with indentation
	prettyJSONData, err := json.MarshalIndent(template.ToJSON(), "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal JSON with indentation: %v", err)
	}

	// Log the pretty-printed JSON
	t.Log(string(prettyJSONData))
}

func projectDirectory(t *testing.T) string {
	wd, _ := os.Getwd()
	for !strings.HasSuffix(wd, "nats-fargate-ddb-cdk-go") {
		wd = filepath.Dir(wd)
	}
	t.Logf("Project directory: %s", wd)
	return wd
}

// createTempFile creates a temporary file, writes the given content to it, and returns the file path
// along with a function that can be used to delete the file.
func createTempFile(t *testing.T, content string) (string, func()) {
	tmpFile, err := os.CreateTemp("", "tempfile-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temporary file: %v", err)
	}

	if _, err := tmpFile.Write([]byte(content)); err != nil {
		tmpFile.Close()
		t.Fatalf("Failed to write to temporary file: %v", err)
	}

	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temporary file: %v", err)
	}

	return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
}

func TestCreateAndDeleteTempFile(t *testing.T) {
	content := "This is a test string."
	filePath, deleteFunc := createTempFile(t, content)

	// Ensure the file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("Temporary file does not exist: %v", err)
	}

	// Delete the temporary file
	deleteFunc()

	// Ensure the file has been deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("Temporary file still exists after deletion: %v", err)
	}
}
