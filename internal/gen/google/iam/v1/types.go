// Package iamv1 re-exports IAM proto types for use by generated grpc-gateway code.
package iamv1

import iampb "cloud.google.com/go/iam/apiv1/iampb"

type SetIamPolicyRequest = iampb.SetIamPolicyRequest
type GetIamPolicyRequest = iampb.GetIamPolicyRequest
type TestIamPermissionsRequest = iampb.TestIamPermissionsRequest
