package policy_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"cri-lite/pkg/policy"
)

var _ = Describe("Image Management Policy", func() {
	var (
		client      runtimeapi.RuntimeServiceClient
		imageClient runtimeapi.ImageServiceClient
		cleanup     func()
	)

	BeforeEach(func() {
		p := policy.NewImageManagementPolicy()
		client, imageClient, cleanup = setupTestEnvironment(p)
	})

	AfterEach(func() {
		cleanup()
	})

	Context("with image management policy", func() {
		It("should allow image calls and deny runtime calls", func() {
			By("calling an image method")
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := imageClient.ListImages(ctx, &runtimeapi.ListImagesRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling a runtime method")
			_, err = client.Version(ctx, &runtimeapi.VersionRequest{})
			Expect(err).NotTo(HaveOccurred())

			By("calling ImageFsInfo")
			_, err = imageClient.ImageFsInfo(ctx, &runtimeapi.ImageFsInfoRequest{})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("with stream interceptor", func() {
		It("should deny GetContainerEvents", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			stream, err := client.GetContainerEvents(ctx, &runtimeapi.GetEventsRequest{})
			Expect(err).NotTo(HaveOccurred())
			_, err = stream.Recv()
			Expect(err).To(HaveOccurred())
			st, ok := status.FromError(err)
			Expect(ok).To(BeTrue())
			Expect(st.Code()).To(Equal(codes.PermissionDenied))
		})
	})
})
