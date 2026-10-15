package engine

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	pb "github.com/gke-labs/extensible-workload-autoscaler/api/proto/v1alpha"
)

// recommendClient serves what processPolicy reads, and records the
// recommendations it pushes. Other methods panic.
type recommendClient struct {
	pb.XASServerClient
	workload    *pb.Workload
	workloadErr error
	pushed      []*pb.UpdateRecommenderStateRequest
}

func (c *recommendClient) GetControlMetrics(context.Context, *pb.GetControlMetricsRequest, ...grpc.CallOption) (*pb.ControlMetrics, error) {
	return &pb.ControlMetrics{}, nil
}

func (c *recommendClient) GetWorkload(context.Context, *pb.GetWorkloadRequest, ...grpc.CallOption) (*pb.Workload, error) {
	return c.workload, c.workloadErr
}

func (c *recommendClient) UpdateRecommenderState(_ context.Context, req *pb.UpdateRecommenderStateRequest, _ ...grpc.CallOption) (*pb.RecommenderState, error) {
	c.pushed = append(c.pushed, req)
	return &pb.RecommenderState{}, nil
}

// workloadRecommender records the workload it is given.
type workloadRecommender struct {
	got []*pb.Workload
}

func (r *workloadRecommender) Recommend(_ *pb.RecommenderDefinition, _, _ *pb.ControlMetrics, workload *pb.Workload) *pb.Recommendation {
	r.got = append(r.got, workload)
	return pb.Recommendation_builder{IsActive: proto.Bool(true)}.Build()
}

func TestProcessPolicyPassesWorkload(t *testing.T) {
	workload := pb.Workload_builder{Pods: []*pb.PodState{
		pb.PodState_builder{Name: proto.String("web-1"), Containers: []*pb.ContainerState{pb.ContainerState_builder{Name: proto.String("app"), Requests: map[string]string{"cpu": "100m"}}.Build()}}.Build(),
	}}.Build()
	policy := pb.Policy_builder{
		Id:         pb.PolicyId_builder{ClusterName: proto.String("default"), Namespace: proto.String("prod"), Name: proto.String("web")}.Build(),
		Activation: []*pb.RecommenderDefinition{pb.RecommenderDefinition_builder{Name: proto.String("act"), Recommender: proto.String("rec-class")}.Build()},
		Scaling:    []*pb.RecommenderDefinition{pb.RecommenderDefinition_builder{Name: proto.String("vpa"), Recommender: proto.String("rec-class")}.Build()},
	}.Build()

	t.Run("every recommender gets the workload", func(t *testing.T) {
		client := &recommendClient{workload: workload}
		rec := &workloadRecommender{}
		e := newTestEngine(t, client, map[string]string{"rec-class": "Rec"}, map[string]Recommender{"Rec": rec})

		e.processPolicy(policy)

		if diff := cmp.Diff([]*pb.Workload{workload, workload}, rec.got, protocmp.Transform(), protocmp.IgnoreDefaultScalars()); diff != "" {
			t.Errorf("workloads given to Recommend mismatch (-want +got):\n%s", diff)
		}
		if len(client.pushed) != 2 {
			t.Errorf("pushed %d recommendations, want 2", len(client.pushed))
		}
	})

	t.Run("the policy is skipped when the workload can't be read", func(t *testing.T) {
		client := &recommendClient{workloadErr: context.DeadlineExceeded}
		rec := &workloadRecommender{}
		e := newTestEngine(t, client, map[string]string{"rec-class": "Rec"}, map[string]Recommender{"Rec": rec})

		e.processPolicy(policy)

		if len(rec.got) != 0 || len(client.pushed) != 0 {
			t.Errorf("Recommend called %d times and %d recommendations pushed, want none", len(rec.got), len(client.pushed))
		}
	})
}
