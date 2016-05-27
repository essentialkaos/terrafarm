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
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// GetActiveBuildNodes return list of nodes with active build process
func GetActiveBuildNodes(prefs *Preferences, maxBuildTime int) []string {
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

		output, err := session.Output(
			fmt.Sprintf("stat -c '%%Y' /home/%s/.buildlock", prefs.User),
		)

		if err == nil {
			if maxBuildTime > 0 {
				buildStartTimestamp, _ := strconv.Atoi(string(output))

				if buildStartTimestamp+maxBuildTime > int(time.Now().Unix()) {
					result = append(result, nodeName)
				}
			} else {
				result = append(result, nodeName)
			}
		}

		session.Close()
		client.Close()
	}

	return result
}
