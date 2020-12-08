// Copyright 2019 Yunion
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

func (ct ComponentType) String() string {
	return string(ct)
}

func (oc *OnecloudCluster) GetRegion() string {
	if len(oc.Status.RegionServer.RegionId) == 0 {
		return oc.Spec.Region
	} else {
		return oc.Status.RegionServer.RegionId
	}
}

func (oc *OnecloudCluster) GetZone(zoneName string) string {
	if len(zoneName) == 0 { // default zone
		if len(oc.Status.RegionServer.ZoneId) == 0 {
			return oc.Spec.Zone
		} else {
			return oc.Status.RegionServer.ZoneId
		}
	} else {
		if zoneId, ok := oc.Status.RegionServer.CustomZones[zoneName]; ok {
			return zoneId
		} else {
			return zoneName
		}
	}
}

func (oc *OnecloudCluster) GetZones() []string {
	var zones = oc.Spec.CustomZones
	if zones == nil {
		zones = []string{}
	}
	return append(zones, oc.Spec.Zone)
}
