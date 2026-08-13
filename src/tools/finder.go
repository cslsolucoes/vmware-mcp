package tools

import (
	"errors"
	"fmt"
	"strings"

	"github.com/vmware/govmomi/find"
)

// dcScopedPath builds an inventory path safe to pass to a Finder list method
// (HostSystemList, DatastoreList, NetworkList, ResourcePoolList,
// ClusterComputeResourceList, VirtualMachineList) regardless of whether the
// client's Finder has a default datacenter set.
//
// vmware.NewClient only calls Finder.SetDatacenter when DefaultDatacenter
// resolves unambiguously (standalone ESXi, or a vCenter with exactly one
// datacenter). Against a vCenter with more than one datacenter it leaves the
// Finder's datacenter unset, and every list method above resolves its root
// folder relative to that datacenter — a bare pattern like "*" then fails
// with "please specify a datacenter" instead of listing anything (confirmed
// against vcsim with Model.Datacenter=2, not assumed).
//
// An absolute path with the datacenter segment wildcarded (e.g.
// "/*/host/*") bypasses that requirement entirely and still resolves
// correctly for a single-datacenter/standalone-ESXi Finder — confirmed
// against both simulator.VPX() (multi-datacenter) and simulator.ESX()
// (standalone). folder is the well-known per-datacenter inventory folder
// name ("vm", "host", "datastore", "network"). If path is already absolute
// (caller wants to scope to a specific datacenter or give a full inventory
// path), it is passed through unchanged.
func dcScopedPath(folder, path string) string {
	if strings.HasPrefix(path, "/") {
		return path
	}
	return fmt.Sprintf("/*/%s/%s", folder, path)
}

// emptyOnNotFound converts govmomi's find.NotFoundError — returned by list
// methods instead of an empty slice when a pattern matches nothing — into a
// nil error, so callers can treat "0 results" as a normal empty list. A
// standalone ESXi host genuinely has 0 clusters and a fresh vCenter can have
// 0 resource pools beyond the implicit ones; that is not a tool failure.
// Any other error is returned unchanged.
func emptyOnNotFound(err error) error {
	var nf *find.NotFoundError
	if errors.As(err, &nf) {
		return nil
	}
	return err
}
