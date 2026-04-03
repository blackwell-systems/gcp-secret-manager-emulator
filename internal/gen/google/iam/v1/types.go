// Package iamv1 re-exports IAM proto types for use by generated grpc-gateway code.
package iamv1

import iampb "google.golang.org/genproto/googleapis/iam/v1"

type SetIamPolicyRequest = iampb.SetIamPolicyRequest
type GetIamPolicyRequest = iampb.GetIamPolicyRequest
type TestIamPermissionsRequest = iampb.TestIamPermissionsRequest
