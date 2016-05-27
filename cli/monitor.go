package cli

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2016 Essential Kaos                         //
//      Essential Kaos Open Source License <http://essentialkaos.com/ekol?en>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"pkg.re/essentialkaos/ek.v1/arg"
	"pkg.re/essentialkaos/ek.v1/fmtc"
	"pkg.re/essentialkaos/ek.v1/fsutil"
	"pkg.re/essentialkaos/ek.v1/jsonutil"
	"pkg.re/essentialkaos/ek.v1/log"
	"pkg.re/essentialkaos/ek.v1/path"
	"pkg.re/essentialkaos/ek.v1/timeutil"
)

// ////////////////////////////////////////////////////////////////////////////////// //

// MonitorState contains monitor specific info
type MonitorState struct {
	Pid          int   `json:"pid"`
	DestroyAfter int64 `json:"destroy_after"`
}

// ////////////////////////////////////////////////////////////////////////////////// //

// startMonitor starts monitoring process
func startMonitor() {
	destroyAfter := time.Unix(int64(arg.GetI(ARG_MONITOR)), 0)
	monitorPid := os.Getpid()

	state := &MonitorState{
		Pid:          monitorPid,
		DestroyAfter: int64(arg.GetI(ARG_MONITOR)),
	}

	if saveMonitorState(state) != nil {
		exit(1)
	}

	log.Set(getMonitorLogFilePath(), 0644)
	log.Aux(SEPARATOR)
	log.Aux("Terrafarm %s monitor started", VER)
	log.Info("Farm will be destroyed after %s", timeutil.Format(destroyAfter, "%Y/%m/%d %H:%M:%S"))

	for {
		if !isTerrafarmActive() {
			log.Info("Farm destroyed manually")
			deleteMonitorState()
			exit(0)
		}

		time.Sleep(time.Minute)

		if time.Now().Unix() <= destroyAfter.Unix() {
			continue
		}

		log.Info("Starting farm destroying...")

		prefs := findAndReadPreferences()
		vars, err := prefsToArgs(prefs, "-no-color", "-force")

		if err != nil {
			continue
		}

		templateDir := path.Join(getDataDir(), prefs.Template)

		fsutil.Push(templateDir)

		err = execTerraform(true, "destroy", vars)

		if err != nil {
			log.Error("Can't destroy farm - terrafarm return error: %v", err)
			continue
		}

		fsutil.Pop()

		os.Remove(getFarmStateFilePath())

		break
	}

	log.Info("Farm successfully destroyed!")

	deleteMonitorState()

	exit(0)
}

// getMonitorLogFilePath return path to monitor log file
func getMonitorLogFilePath() string {
	return path.Join(getDataDir(), MONITOR_LOG_FILE)
}

// getMonitorStateFilePath return path to monitor state file
func getMonitorStateFilePath() string {
	return path.Join(getDataDir(), MONITOR_STATE_FILE)
}

// saveMonitorState save monitor state to file
func saveMonitorState(state *MonitorState) error {
	return jsonutil.EncodeToFile(getMonitorStateFilePath(), state)
}

func deleteMonitorState() error {
	return os.Remove(getMonitorStateFilePath())
}

// readMonitorDestroyDate read monitor state from file
func readMonitorState() (*MonitorState, error) {
	state := &MonitorState{}
	stateFile := getMonitorStateFilePath()

	if !fsutil.IsExist(stateFile) {
		return nil, fmt.Errorf("Monitor state file is not exist")
	}

	err := jsonutil.DecodeFile(stateFile, state)

	if err != nil {
		return nil, err
	}

	return state, nil
}

// runMonitor run monitoring process
func runMonitor(ttl int64) error {
	destroyTime := time.Now().Unix() + (ttl * 60)

	cmd := exec.Command("terrafarm", "--monitor", fmtc.Sprintf("%d", destroyTime))

	return cmd.Start()
}

// isMonitorActive return true is monitor process is active
func isMonitorActive() bool {
	state, err := readMonitorState()

	if err != nil {
		return false
	}

	return fsutil.IsExist(path.Join("/proc", fmtc.Sprintf("%d", state.Pid)))
}

// ////////////////////////////////////////////////////////////////////////////////// //
