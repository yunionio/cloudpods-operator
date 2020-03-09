package image

import (
	"reflect"
	"testing"
)

func TestParseImageReference(t *testing.T) {
	type args struct {
		img string
	}
	tests := []struct {
		name    string
		args    args
		want    *ImageReference
		wantErr bool
	}{
		{
			"yunion/image",
			args{"yunion/image"},
			&ImageReference{
				Domain:     "yunion",
				Repository: "yunion",
				Image:      "image",
				Tag:        "latest",
			},
			false,
		},
		{
			"10.168.222.173:8082/yunion/image:v2",
			args{"10.168.222.173:8082/yunion/image:v2"},
			&ImageReference{
				Domain:     "10.168.222.173:8082",
				Repository: "10.168.222.173:8082/yunion",
				Image:      "image",
				Tag:        "v2",
			},
			false,
		},
		{
			"test tag with digest",
			args{"registry.cn-beijing.aliyuncs.com/yunionio/climc:v1@sha256:32dcddaa6271b8c752bd6574c789771d38315076ce300c4a1d4618496e359f2d"},
			&ImageReference{
				Domain:     "registry.cn-beijing.aliyuncs.com",
				Repository: "registry.cn-beijing.aliyuncs.com/yunionio",
				Image:      "climc",
				Tag:        "v1",
				Digest:     "sha256:32dcddaa6271b8c752bd6574c789771d38315076ce300c4a1d4618496e359f2d",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseImageReference(tt.args.img)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseImageReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseImageReference() = %v, want %v", got, tt.want)
			}
		})
	}
}
