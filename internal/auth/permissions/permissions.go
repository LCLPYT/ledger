package permissions

import (
	"maps"
	"slices"

	apiv1 "github.com/lclpyt/ledger/internal/api/v1"
	"github.com/lclpyt/ledger/internal/assetrepo"
	"github.com/lclpyt/ledger/internal/auth"
	"github.com/samber/do/v2"
)

// ListAvailable returns a sorted list of all known permissions used in the app
func ListAvailable(i do.Injector) ([]string, error) {
	methods, err := apiv1.MethodPermissions()
	if err != nil {
		return nil, err
	}

	assets, err := assetrepo.Permissions(i)
	if err != nil {
		return nil, err
	}

	set := make(map[string]struct{})

	for _, permission := range methods {
		if permission != auth.Public {
			set[permission] = struct{}{}
		}
	}

	for _, permission := range assets {
		set[permission] = struct{}{}
	}

	return slices.Sorted(maps.Keys(set)), nil
}
