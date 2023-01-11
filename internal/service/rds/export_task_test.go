package rds_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	rdsv1 "github.com/aws/aws-sdk-go/service/rds"
	sdkacctest "github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-aws/internal/acctest"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/create"
	tfrds "github.com/hashicorp/terraform-provider-aws/internal/service/rds"
	"github.com/hashicorp/terraform-provider-aws/names"
)

func TestAccRDSExportTask_basic(t *testing.T) {
	var exportTask types.ExportTask
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_rds_export_task.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			acctest.PreCheckPartitionHasService(rdsv1.EndpointsID, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, rdsv1.EndpointsID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckExportTaskDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccExportTaskConfig_basic(rName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckExportTaskExists(resourceName, &exportTask),
					resource.TestCheckResourceAttr(resourceName, "export_task_identifier", rName),
					resource.TestCheckResourceAttr(resourceName, "id", rName),
					resource.TestCheckResourceAttrPair(resourceName, "source_arn", "aws_db_snapshot.test", "db_snapshot_arn"),
					resource.TestCheckResourceAttrPair(resourceName, "s3_bucket_name", "aws_s3_bucket.test", "id"),
					resource.TestCheckResourceAttrPair(resourceName, "iam_role_arn", "aws_iam_role.test", "arn"),
					resource.TestCheckResourceAttrPair(resourceName, "kms_key_id", "aws_kms_key.test", "arn"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccRDSExportTask_optional(t *testing.T) {
	var exportTask types.ExportTask
	rName := sdkacctest.RandomWithPrefix(acctest.ResourcePrefix)
	resourceName := "aws_rds_export_task.test"
	s3Prefix := "test_prefix/test-export"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			acctest.PreCheckPartitionHasService(rdsv1.EndpointsID, t)
		},
		ErrorCheck:               acctest.ErrorCheck(t, rdsv1.EndpointsID),
		ProtoV5ProviderFactories: acctest.ProtoV5ProviderFactories,
		CheckDestroy:             testAccCheckExportTaskDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccExportTaskConfig_optional(rName, s3Prefix),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckExportTaskExists(resourceName, &exportTask),
					resource.TestCheckResourceAttr(resourceName, "export_task_identifier", rName),
					resource.TestCheckResourceAttr(resourceName, "id", rName),
					resource.TestCheckResourceAttrPair(resourceName, "source_arn", "aws_db_snapshot.test", "db_snapshot_arn"),
					resource.TestCheckResourceAttrPair(resourceName, "s3_bucket_name", "aws_s3_bucket.test", "id"),
					resource.TestCheckResourceAttrPair(resourceName, "iam_role_arn", "aws_iam_role.test", "arn"),
					resource.TestCheckResourceAttrPair(resourceName, "kms_key_id", "aws_kms_key.test", "arn"),
					resource.TestCheckResourceAttr(resourceName, "export_only.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "export_only.0", "database"),
					resource.TestCheckResourceAttr(resourceName, "s3_prefix", s3Prefix),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCheckExportTaskDestroy(s *terraform.State) error {
	ctx := context.Background()
	conn := acctest.Provider.Meta().(*conns.AWSClient).RDSClient()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_rds_export_task" {
			continue
		}

		out, err := tfrds.FindExportTaskByID(ctx, conn, rs.Primary.ID)
		if err != nil {
			var nfe *resource.NotFoundError
			if errors.As(err, &nfe) {
				return nil
			}
			return err
		}
		if !isInDestroyedStatus(aws.ToString(out.Status)) {
			return create.Error(names.RDS, create.ErrActionCheckingDestroyed, tfrds.ResNameExportTask, rs.Primary.ID, errors.New("not destroyed"))
		}
	}

	return nil
}

func testAccCheckExportTaskExists(name string, exportTask *types.ExportTask) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return create.Error(names.RDS, create.ErrActionCheckingExistence, tfrds.ResNameExportTask, name, errors.New("not found"))
		}

		if rs.Primary.ID == "" {
			return create.Error(names.RDS, create.ErrActionCheckingExistence, tfrds.ResNameExportTask, name, errors.New("not set"))
		}

		ctx := context.Background()
		conn := acctest.Provider.Meta().(*conns.AWSClient).RDSClient()
		resp, err := tfrds.FindExportTaskByID(ctx, conn, rs.Primary.ID)
		if err != nil {
			return create.Error(names.RDS, create.ErrActionCheckingExistence, tfrds.ResNameExportTask, rs.Primary.ID, err)
		}

		*exportTask = *resp

		return nil
	}
}

// isInDestroyedStatus determines whether the export task status is a value that could
// be returned if the resource was properly destroyed.
//
// COMPLETED and FAILED statuses are valid because the resource is simply removed from
// state in these scenarios. In-progress tasks should be cancelled upon destroy, so CANCELED
// and CANCELLING are also valid.
func isInDestroyedStatus(s string) bool {
	// AWS does not provide enum types for these statuses
	deletedStatuses := []string{"CANCELED", "CANCELLING", "COMPLETED", "FAILED"}
	for _, status := range deletedStatuses {
		if s == status {
			return true
		}
	}
	return false
}

func testAccExportTaskConfigBase(rName string) string {
	return fmt.Sprintf(`
resource "aws_s3_bucket" "test" {
  bucket        = %[1]q
  force_destroy = true
}

resource "aws_s3_bucket_acl" "test" {
  bucket = aws_s3_bucket.test.id
  acl    = "private"
}

resource "aws_iam_role" "test" {
  name = %[1]q

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Sid    = ""
        Principal = {
          Service = "export.rds.amazonaws.com"
        }
      },
    ]
  })
}

data "aws_iam_policy_document" "test" {
  statement {
    actions = [
      "s3:ListAllMyBuckets",
    ]
    resources = [
      "*"
    ]
  }
  statement {
    actions = [
      "s3:GetBucketLocation",
      "s3:ListBucket",
    ]
    resources = [
      aws_s3_bucket.test.arn,
    ]
  }
  statement {
    actions = [
      "s3:GetObject",
      "s3:PutObject",
      "s3:DeleteObject",
    ]
    resources = [
      "${aws_s3_bucket.test.arn}/*"
    ]
  }
}

resource "aws_iam_policy" "test" {
  name   = %[1]q
  policy = data.aws_iam_policy_document.test.json
}

resource "aws_iam_role_policy_attachment" "test-attach" {
  role       = aws_iam_role.test.name
  policy_arn = aws_iam_policy.test.arn
}

resource "aws_kms_key" "test" {
  deletion_window_in_days = 10
}

resource "aws_db_instance" "test" {
  identifier           = %[1]q
  allocated_storage    = 10
  db_name              = "test"
  engine               = "mysql"
  engine_version       = "5.7"
  instance_class       = "db.t3.micro"
  username             = "foo"
  password             = "foobarbaz"
  parameter_group_name = "default.mysql5.7"
  skip_final_snapshot  = true
}

resource "aws_db_snapshot" "test" {
  db_instance_identifier = aws_db_instance.test.id
  db_snapshot_identifier = %[1]q
}
`, rName)
}

func testAccExportTaskConfig_basic(rName string) string {
	return acctest.ConfigCompose(
		testAccExportTaskConfigBase(rName),
		fmt.Sprintf(`
resource "aws_rds_export_task" "test" {
  export_task_identifier = %[1]q
  source_arn             = aws_db_snapshot.test.db_snapshot_arn
  s3_bucket_name         = aws_s3_bucket.test.id
  iam_role_arn           = aws_iam_role.test.arn
  kms_key_id             = aws_kms_key.test.arn
}
`, rName))
}

func testAccExportTaskConfig_optional(rName, s3Prefix string) string {
	return acctest.ConfigCompose(
		testAccExportTaskConfigBase(rName),
		fmt.Sprintf(`
resource "aws_rds_export_task" "test" {
  export_task_identifier = %[1]q
  source_arn             = aws_db_snapshot.test.db_snapshot_arn
  s3_bucket_name         = aws_s3_bucket.test.id
  iam_role_arn           = aws_iam_role.test.arn
  kms_key_id             = aws_kms_key.test.arn

  export_only = ["database"]
  s3_prefix   = %[2]q
}
`, rName, s3Prefix))
}
