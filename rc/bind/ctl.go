package bind

import "os"

const SelfCtlPath = "#task/self/ctl"

func WriteSelfCtl(msg string) error {
	f, err := os.OpenFile(SelfCtlPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write([]byte(msg + "\n"))
	return err
}
