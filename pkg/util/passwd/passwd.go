package passwd

import "yunion.io/x/pkg/util/seclib"

func GeneratePassword() string {
	return seclib.RandomPassword(16)
}
