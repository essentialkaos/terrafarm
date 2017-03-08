package cli

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2017 ESSENTIAL KAOS                         //
//        Essential Kaos Open Source License <https://essentialkaos.com/ekol>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"fmt"
	"io/ioutil"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/essentialkaos/terrafarm/prefs"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// getBuildNodesInfo return list of with info about build nodes
func getBuildNodesInfo(p *prefs.Preferences) []*NodeInfo {
	keyData, err := ioutil.ReadFile(p.Key)

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

	nodes, err := collectNodesInfo(p)

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
			fmt.Sprintf("stat -c '%%Y' /home/%s/.buildlock", p.User),
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
func getActiveBuildNodesCount(p *prefs.Preferences) int {
	nodes := getBuildNodesInfo(p)

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
func getActiveBuildNodesNames(p *prefs.Preferences) []string {
	nodes := getBuildNodesInfo(p)

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
