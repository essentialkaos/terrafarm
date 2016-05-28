package cli

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2016 Essential Kaos                         //
//      Essential Kaos Open Source License <http://essentialkaos.com/ekol?en>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"fmt"
	"io/ioutil"
	"time"

	"golang.org/x/crypto/ssh"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// GetActiveBuildNodes return list of nodes with active build process
func GetActiveBuildNodes(prefs *Preferences) []string {
	keyData, err := ioutil.ReadFile(prefs.Key)

	if err != nil {
		fmt.Println(err.Error())
		return []string{}
	}

	signer, err := ssh.ParsePrivateKey(keyData)

	if err != nil {
		fmt.Println(err.Error())
		return []string{}
	}

	sshConfig := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		Timeout: time.Second,
	}

	var result []string

	nodes, err := getNodeList(prefs)

	if err != nil {
		fmt.Println(err.Error())
		return []string{}
	}

	for nodeName, nodeIP := range nodes {
		client, err := ssh.Dial("tcp", nodeIP+":22", sshConfig)

		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		session, err := client.NewSession()

		if err != nil {
			fmt.Println(err.Error())
			client.Close()
			continue
		}

		_, err = session.Output(
			fmt.Sprintf("stat -c '%%Y' /home/%s/.buildlock", prefs.User),
		)

		if err == nil {
			result = append(result, nodeName)
		}

		session.Close()
		client.Close()
	}

	return result
}
