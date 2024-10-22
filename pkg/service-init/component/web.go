package component

import (
	"fmt"
	"path/filepath"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
)

func init() {
	RegisterComponent(NewWeb())
}

type web struct {
	*baseService
}

func NewWeb() Component {
	return &web{newBaseService(v1alpha1.WebComponentType, nil)}
}

const (
	CEConfig = `
    location / {
        root /usr/share/nginx/html/web;
        index index.html;
        add_header Cache-Control no-cache;
        expires 1s;
        if (!-e $request_filename) {
            rewrite ^/(.*) /index.html last;
            break;
        }
    }
`

	EEConfig = `
    location ~ ^/v[12]/ { rewrite ^/v.(.*)$ $1 redirect; }

    location / {
        root   /usr/share/nginx/html/dashboard;
        index index.html;
        try_files $uri $uri/ /index.html;
    }

    location /overview {
        proxy_pass http://localhost:8080;
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
   }

    location /docs {
        proxy_pass http://localhost:8081;
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
   }
`

	WebNginxConfigTemplate = `
map $http_upgrade $connection_upgrade {
    default upgrade;
    '' close;
}

server {
	{{- if .UseHTTP}}
    listen 80 default_server;
    server_name _;
	{{- else }}
    listen 443 default_server ssl;
    server_name _;
    ssl_certificate /etc/yunion/pki/service.crt;
    ssl_certificate_key /etc/yunion/pki/service.key;
	{{- end }}

    gzip_static on;
    gzip on;
    gzip_proxied any;
    gzip_min_length  1k;
    gzip_buffers     4 16k;
    gzip_http_version 1.0;
    gzip_comp_level 5;
    gzip_types text/plain application/javascript application/css text/css application/xml application/json text/javascript application/x-httpd-php image/jpeg image/gif image/png;
    gzip_vary on;
    chunked_transfer_encoding off;

    client_body_buffer_size 16k;
    client_header_buffer_size 16k;
    client_max_body_size 8m;
    large_client_header_buffers 2 16k;
    client_body_timeout 20s;
    client_header_timeout 120s;

{{.EditionConfig}}

    location /static/ {
        # Some basic cache-control for static files to be sent to the browser
        root /usr/share/nginx/html/web;
        expires max;
        add_header Pragma public;
        add_header Cache-Control "public, must-revalidate, proxy-revalidate";
    }

    location /itsm {
        alias /usr/share/nginx/html/itsm;
        index index.html;
        add_header Cache-Control no-cache;
        expires 1s;
        if (!-e $request_filename) {
            rewrite ^/(.*) /itsm/index.html last;
            break;
        }
    }

    location /static-itsm/ {
        root /usr/share/nginx/html/itsm;
        expires max;
        add_header Pragma public;
        add_header Cache-Control "public, must-revalidate, proxy-revalidate";
    }

    location ~* /static/images/favicon.* {
        root /usr/share/nginx/html/login;
        expires max;
        add_header Pragma public;
        add_header Cache-Control "public, must-revalidate, proxy-revalidate";
    }

    location /api {
        proxy_pass {{.APIGatewayURL}};
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_buffer_size  16k;
        proxy_buffers   32 16k;
        proxy_busy_buffers_size 16k;
        proxy_temp_file_write_size 16k;
        proxy_read_timeout 600;
    }

    location /api/v1/imageutils/upload {
        proxy_pass {{.APIGatewayURL}};
        client_max_body_size 0;
        client_body_timeout 300;
        proxy_http_version 1.1;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;
    }

    location /api/v1/s3uploads {
        proxy_pass {{.APIGatewayURL}};
        client_max_body_size 0;
        client_body_timeout 300;
        proxy_http_version 1.1;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;
    }

    location /query {
        proxy_pass {{.APIGatewayURL}};
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location /api/v1/webconsole/sftp {
        client_max_body_size 0;
        proxy_http_version 1.1;
        proxy_request_buffering off;
        proxy_buffering off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $remote_addr;
        rewrite ^/api/v1/(.*)$ /$1 break;
        proxy_pass {{.WebconsoleURL}};
    }

    location ~ ^/(vnc|spice|wmks|sol) {
        proxy_pass {{.WebconsoleURL}};
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    }

    location ~ ^/(websockify|wsproxy|connect) {
        proxy_pass {{.WebconsoleURL}};
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 86400;
        proxy_send_timeout 86400;
    }

    location /web-console {
        alias /usr/share/nginx/html/web-console;
        index index.html;
        if (!-e $request_filename) {
            rewrite ^/(.*) /web-console/index.html last;
            break;
        }
    }

    location /ws {
        proxy_pass {{.APIGatewayWsURL}};
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 86400;
    }

    location /socket.io/ {
        proxy_pass {{.APIGatewayWsURL}};
        proxy_redirect   off;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 86400;
    }

    location /baremetal-prepare/ {
        # Some basic cache-control for static files to be sent to the browser
        root /opt/cloud/yunion/baremetal/;
        expires max;
        add_header Pragma public;
        add_header Cache-Control "public, must-revalidate, proxy-revalidate";
    }
}
`
)

type WebNginxConfig struct {
	EditionConfig   string
	WebconsoleURL   string
	APIGatewayWsURL string
	APIGatewayURL   string
	UseHTTP         bool
}

func (c WebNginxConfig) GetContent() (string, error) {
	return CompileTemplateFromMap(WebNginxConfigTemplate, c)
}

func (w web) GetConfigFilePath(targetDir string) string {
	return filepath.Join(targetDir, "/etc/nginx/conf.d/default.conf")
}

func (w web) GetConfig(oc *v1alpha1.OnecloudCluster, cfg *v1alpha1.OnecloudClusterConfig) (interface{}, error) {
	urlF := func(ct v1alpha1.ComponentType, port int) string {
		return fmt.Sprintf("https://%s:%d", controller.NewClusterComponentName(oc.GetName(), ct), port)
	}
	conf := CEConfig
	isEE := v1alpha1.IsEEOrESEEdition(oc)
	if isEE {
		conf = EEConfig
	}
	config := WebNginxConfig{
		EditionConfig:   conf,
		WebconsoleURL:   urlF(v1alpha1.WebconsoleComponentType, constants.WebconsolePort),
		APIGatewayWsURL: urlF(v1alpha1.APIGatewayComponentType, constants.APIWebsocketPort),
		APIGatewayURL:   urlF(v1alpha1.APIGatewayComponentType, constants.APIGatewayPort),
		UseHTTP:         oc.Spec.Web.UseHTTP,
	}
	return config.GetContent()
}
