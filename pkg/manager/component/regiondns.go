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

const (
	RegionDNSConfigTemplate = `
.:53 {
    cache 30
    yunion . {
        sql_connection mysql+pymysql://{{.DBUser}}:{{.DBPassword}}@{{.DBHost}}:{{.DBPort}}/{{.DBName}}?charset=utf8
        dns_domain {{.Domain}}
        region {{.Region}}
        auth_url {{.AuthURL}}
        admin_project system
        admin_user {{.AdminUser}}
        admin_password {{.AdminPassword}}
        fallthrough .
    }
    proxy . {{.ProxyServer}}:53
    log {
        class error
    }
}
`
)
