//go:build darwin
// +build darwin

/*
Copyright 2023 Amidaware Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import _ "embed"

//go:embed scripts/macos_fix_mesh_install.sh
var ventura_mesh_fix string

func (a *Agent) FixVenturaMesh() {
	a.RunScript(ventura_mesh_fix, "foo", []string{}, 45, false, []string{}, false, "")
}
