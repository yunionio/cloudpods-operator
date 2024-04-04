package component

import (
	"k8s.io/klog"

	"yunion.io/x/onecloud/pkg/mcclient"
	"yunion.io/x/pkg/errors"

	"yunion.io/x/onecloud-operator/pkg/apis/constants"
	"yunion.io/x/onecloud-operator/pkg/apis/onecloud/v1alpha1"
	"yunion.io/x/onecloud-operator/pkg/controller"
	"yunion.io/x/onecloud-operator/pkg/util/onecloud"
)

func LoginByServiceAccount(s *mcclient.ClientSession, account v1alpha1.CloudUser) (mcclient.TokenCredential, error) {
	return s.GetClient().AuthenticateWithSource(account.Username, account.Password, constants.DefaultDomain, constants.SysAdminProject, "", "operator")
}

func EnsureServiceAccount(s *mcclient.ClientSession, account v1alpha1.CloudUser) error {
	username := account.Username
	password := account.Password
	obj, exists, err := onecloud.IsUserExists(s, username)
	if err != nil {
		return err
	}
	if exists {
		if userProjectCnt, err := obj.Int("project_count"); err != nil {
			klog.Errorf("Get user %s project_count: %v", username, err)
		} else {
			if userProjectCnt == 0 {
				userId, _ := obj.GetString("id")
				if err := onecloud.ProjectAddUser(s, constants.SysAdminProject, userId, constants.RoleAdmin); err != nil {
					return errors.Wrapf(err, "add exists user %s to system project", username)
				}
			}
		}
		if !controller.SyncUser {
			return nil
		} else {
			// no need to check, just update QJ
			// password not change
			// if _, err := LoginByServiceAccount(s, account); err == nil {
			//	return nil
			// }
			id, _ := obj.GetString("id")
			if _, err := onecloud.ChangeUserPassword(s, id, password); err != nil {
				return errors.Wrapf(err, "user %s(%s) already exists, update password", username, id)
			}
			return nil
		}
	}
	obj, err = onecloud.CreateUser(s, username, password)
	if err != nil {
		return errors.Wrapf(err, "create user %s", username)
	}
	userId, _ := obj.GetString("id")
	return onecloud.ProjectAddUser(s, constants.SysAdminProject, userId, constants.RoleAdmin)
}
