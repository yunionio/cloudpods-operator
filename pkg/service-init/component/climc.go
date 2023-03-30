package component

import (
	"os"

	"yunion.io/x/jsonutils"
	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/onecloud/pkg/mcclient/modules/identity"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
)

func init() {
	RegisterComponent(NewClimc())
}

type climc struct {
	*baseService
}

const (
	ENV_DEFAULT_CLIMC_USER          = "CLIMC_DEFAULT_USER"
	ENV_DEFAULT_CLIMC_USER_PASSWORD = "CLIMC_DEFAULT_USER_PASSWORD"
)

func NewClimc() Component {
	return &climc{
		baseService: newBaseService(v1alpha1.ClimcComponentType, nil),
	}
}

func (c climc) BeforeStart(oc *v1alpha1.OnecloudCluster, targetCfgDir string) error {
	user, _ := os.LookupEnv(ENV_DEFAULT_CLIMC_USER)
	userPwd, _ := os.LookupEnv(ENV_DEFAULT_CLIMC_USER_PASSWORD)
	if user == "" || userPwd == "" {
		return nil
	}
	return controller.RunWithSession(oc, func(s *mcclient.ClientSession) error {
		if err := EnsureServiceAccount(s, v1alpha1.CloudUser{
			Username: user,
			Password: userPwd,
		}); err != nil {
			return errors.Wrap(err, "sync climc user")
		}

		userOpt := map[string]interface{}{
			"allow_web_console": true,
			"is_system_account": false,
			"enabled":           true,
		}
		if _, err := identity.UsersV3.Update(s, user, jsonutils.Marshal(userOpt)); err != nil {
			return errors.Wrapf(err, "allow user %q login from web", user)
		}
		return nil
	})
}
