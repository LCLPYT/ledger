package auth

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	authv1 "github.com/lclpyt/ledger/gen/go/apis/auth/v1"
)

// PackagePermissions reads the method permissions of every service in the registered proto package.
func PackagePermissions(pkg protoreflect.FullName) (MethodPermissions, error) {
	var files []protoreflect.FileDescriptor

	protoregistry.GlobalFiles.RangeFilesByPackage(pkg, func(file protoreflect.FileDescriptor) bool {
		files = append(files, file)

		return true
	})

	if len(files) == 0 {
		return nil, fmt.Errorf("auth: no proto files registered for package %s", pkg)
	}

	return FilePermissions(files...)
}

// FilePermissions reads the apis.auth.v1.requirement option of every method the files declare.
// Every method must declare either a concrete permission or public access.
func FilePermissions(files ...protoreflect.FileDescriptor) (MethodPermissions, error) {
	permissions := MethodPermissions{}

	var errs []error

	for _, file := range files {
		services := file.Services()

		for i := range services.Len() {
			methods := services.Get(i).Methods()

			for j := range methods.Len() {
				method := methods.Get(j)

				permission, err := methodPermission(method)

				if err != nil {
					errs = append(errs, fmt.Errorf("auth: method %s: %w", method.FullName(), err))

					continue
				}

				permissions[fmt.Sprintf("/%s/%s", method.Parent().FullName(), method.Name())] = permission
			}
		}
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return permissions, nil
}

func methodPermission(method protoreflect.MethodDescriptor) (string, error) {
	requirement, _ := proto.GetExtension(method.Options(), authv1.E_Requirement).(*authv1.Requirement)

	switch r := requirement.GetRequirement().(type) {
	case *authv1.Requirement_Permission:
		if !ValidPermission(r.Permission) || strings.Contains(r.Permission, "*") {
			return "", fmt.Errorf("invalid permission %q", r.Permission)
		}

		return r.Permission, nil
	case *authv1.Requirement_Public:
		if !r.Public {
			return "", errors.New("public must be true when set")
		}

		return Public, nil
	default:
		return "", errors.New("missing apis.auth.v1.requirement option")
	}
}
