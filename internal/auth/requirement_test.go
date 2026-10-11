package auth_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/emptypb"

	authv1 "github.com/lclpyt/ledger/gen/go/apis/auth/v1"
	"github.com/lclpyt/ledger/internal/auth"
)

// serviceFile builds a file declaring one service with a method per requirement, nil leaves the option unset.
func serviceFile(requirements map[string]*authv1.Requirement) protoreflect.FileDescriptor {
	GinkgoHelper()

	service := &descriptorpb.ServiceDescriptorProto{Name: new("TestService")}

	for name, requirement := range requirements {
		options := &descriptorpb.MethodOptions{}

		if requirement != nil {
			proto.SetExtension(options, authv1.E_Requirement, requirement)
		}

		service.Method = append(service.Method, &descriptorpb.MethodDescriptorProto{
			Name:       new(name),
			InputType:  new(".google.protobuf.Empty"),
			OutputType: new(".google.protobuf.Empty"),
			Options:    options,
		})
	}

	file, err := protodesc.NewFile(&descriptorpb.FileDescriptorProto{
		Name:       new("test/v1/test.proto"),
		Package:    new("test.v1"),
		Syntax:     new("proto3"),
		Dependency: []string{emptypb.File_google_protobuf_empty_proto.Path()},
		Service:    []*descriptorpb.ServiceDescriptorProto{service},
	}, protoregistry.GlobalFiles)
	Expect(err).ToNot(HaveOccurred())

	return file
}

func permission(p string) *authv1.Requirement {
	return &authv1.Requirement{Requirement: &authv1.Requirement_Permission{Permission: p}}
}

func public(p bool) *authv1.Requirement {
	return &authv1.Requirement{Requirement: &authv1.Requirement_Public{Public: p}}
}

var _ = Describe("FilePermissions", func() {
	It("maps every method to its declared requirement", func() {
		permissions, err := auth.FilePermissions(serviceFile(map[string]*authv1.Requirement{
			"Read":   permission("games.read"),
			"Status": public(true),
		}))

		Expect(err).ToNot(HaveOccurred())
		Expect(permissions).To(Equal(auth.MethodPermissions{
			"/test.v1.TestService/Read":   "games.read",
			"/test.v1.TestService/Status": auth.Public,
		}))
	})

	DescribeTable("rejects an invalid requirement",
		func(requirement *authv1.Requirement, message string) {
			_, err := auth.FilePermissions(serviceFile(map[string]*authv1.Requirement{
				"Valid":   permission("games.read"),
				"Invalid": requirement,
			}))

			Expect(err).To(MatchError(ContainSubstring("test.v1.TestService.Invalid")))
			Expect(err).To(MatchError(ContainSubstring(message)))
		},
		Entry("missing option", nil, "missing"),
		Entry("empty requirement", &authv1.Requirement{}, "missing"),
		Entry("empty permission", permission(""), "invalid permission"),
		Entry("malformed permission", permission("Games Read"), "invalid permission"),
		Entry("wildcard permission", permission("games.*"), "invalid permission"),
		Entry("public set to false", public(false), "public must be true"),
	)

	It("reports every invalid method at once", func() {
		_, err := auth.FilePermissions(serviceFile(map[string]*authv1.Requirement{
			"First":  nil,
			"Second": nil,
		}))

		Expect(err).To(MatchError(ContainSubstring("TestService.First")))
		Expect(err).To(MatchError(ContainSubstring("TestService.Second")))
	})
})
