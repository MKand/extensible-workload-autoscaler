package policy

import (
	"fmt"
	"hash/fnv"
	"strconv"

	pb "github.com/gke-labs/extensible-workload-autoscaler/api/proto/v1alpha"
	"google.golang.org/protobuf/proto"
)

// CreateEtag computes a non-cryptographic 64-bit FNV-1a hash of policy and sets policy.Etag.
func CreateEtag(policy *pb.Policy) error {
	if policy == nil {
		return fmt.Errorf("cannot compute Etag on nil policy")
	}
	oldEtag := policy.Etag
	policy.Etag = ""
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(policy)
	if err != nil {
		policy.Etag = oldEtag
		return fmt.Errorf("unable to marshal policy to bytes: %w", err)
	}
	h := fnv.New64a()
	h.Write(b)
	policy.Etag = strconv.FormatUint(h.Sum64(), 16)
	return nil
}
