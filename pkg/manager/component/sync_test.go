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

package component

import "testing"

func Test_getRepoImageName(t *testing.T) {
	tests := []struct {
		name  string
		img   string
		want  string
		want1 string
		want2 string
	}{
		{
			name:  "yunionio/util",
			img:   "yunionio/util",
			want:  "yunionio",
			want1: "util",
			want2: "latest",
		},
		{
			name:  "registry.cn-beijing.aliyuncs.com/yunionio/web-ee:v2.12.0",
			img:   "registry.cn-beijing.aliyuncs.com/yunionio/web-ee:v2.12.0",
			want:  "registry.cn-beijing.aliyuncs.com/yunionio",
			want1: "web-ee",
			want2: "v2.12.0",
		},
		{
			name:  "10.127.10.156:5000/yunionio/glance:v3.0.0-20200105.0",
			img:   "10.127.10.156:5000/yunionio/glance:v3.0.0-20200105.0",
			want:  "10.127.10.156:5000/yunionio",
			want1: "glance",
			want2: "v3.0.0-20200105.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2 := getRepoImageName(tt.img)
			if got != tt.want {
				t.Errorf("getRepoImageName() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("getRepoImageName() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("getRepoImageName() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}
