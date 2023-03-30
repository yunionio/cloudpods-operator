package component

import (
	"reflect"
	"testing"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
)

func TestTransSQLAchemyURL(t *testing.T) {
	type args struct {
		pySQLSrc string
	}
	tests := []struct {
		name    string
		args    args
		want    *v1alpha1.DBConfig
		wantErr bool
	}{
		{
			name: "test",
			args: args{pySQLSrc: "mysql+pymysql://keystone:Y35ZZmgrMJBjyVBV@mysql:3306/keystone?charset=utf8"},
			want: &v1alpha1.DBConfig{
				Database: "keystone",
				Username: "keystone",
				Password: "Y35ZZmgrMJBjyVBV",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSQLAchemyURL(tt.args.pySQLSrc)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSQLAchemyURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseSQLAchemyURL() got = %v, want %v", got, tt.want)
			}
		})
	}
}
