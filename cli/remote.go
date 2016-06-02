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

// getBuildNodesInfo return list of with info about build nodes
func getBuildNodesInfo(prefs *Preferences) []*NodeInfo {
	keyData, err := ioutil.ReadFile(prefs.Key)

	if err != nil {
		fmt.Println(err.Error())
		return []*NodeInfo{}
	}

	signer, err := ssh.ParsePrivateKey(keyData)

	if err != nil {
		fmt.Println(err.Error())
		return []*NodeInfo{}
	}

	sshConfig := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		Timeout: time.Second,
	}

	nodes, err := collectNodesInfo(prefs)

	if err != nil {
		fmt.Println(err.Error())
		return []*NodeInfo{}
	}

	for _, node := range nodes {
		client, err := ssh.Dial("tcp", node.IP+":22", sshConfig)

		if err != nil {
			node.State = STATE_DOWN
			continue
		}

		session, err := client.NewSession()

		if err != nil {
			node.State = STATE_DOWN
			client.Close()
			continue
		}

		_, err = session.Output(
			fmt.Sprintf("stat -c '%%Y' /home/%s/.buildlock", prefs.User),
		)

		if err == nil {
			node.State = STATE_ACTIVE
		} else {
			node.State = STATE_INACTIVE
		}

		session.Close()
		client.Close()
	}

	return nodes
}

// getActiveBuildNodesCount return number of build nodes with active
// build process
func getActiveBuildNodesCount(prefs *Preferences) int {
	nodes := getBuildNodesInfo(prefs)

	if len(nodes) == 0 {
		return 0
	}

	result := 0

	for _, node := range nodes {
		if node.State == STATE_ACTIVE {
			result++
		}
	}

	return result
}

// getActiveBuildNodesNames return slice with names of build nodes
// with active build process
func getActiveBuildNodesNames(prefs *Preferences) []string {
	nodes := getBuildNodesInfo(prefs)

	if len(nodes) == 0 {
		return []string{}
	}

	var result []string

	for _, node := range nodes {
		if node.State == STATE_ACTIVE {
			result = append(result, node.Name)
		}
	}

	return result
}
