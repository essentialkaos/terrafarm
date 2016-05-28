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
	"strconv"
	"strings"
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
	Started      int64 `json:"started"`
	DestroyAfter int64 `json:"destroy_after"`
	MaxWait      int64 `json:"max_wait"`
}

// ////////////////////////////////////////////////////////////////////////////////// //

// startMonitor starts monitoring process
func startMonitor() {
	log.Set(getMonitorLogFilePath(), 0644)
	log.Aux(SEPARATOR)
	log.Aux("Terrafarm %s monitor started", VER)

	destroyAfter, maxWait, ok := parseMonitoringPreferences(arg.GetS(ARG_MONITOR))

	if !ok {
		log.Crit("Can't parse given monitor preferences (%s)", arg.GetS(ARG_MONITOR))
		exit(1)
	}

	monitorPid := os.Getpid()

	state := &MonitorState{
		Pid:          monitorPid,
		Started:      time.Now().Unix(),
		DestroyAfter: destroyAfter.Unix(),
		MaxWait:      maxWait,
	}

	err := saveMonitorState(state)

	if err != nil {
		log.Crit("Can't save monitor state to file: %v", err)
		exit(1)
	}

	runMonitoringLoop(destroyAfter, maxWait)

	deleteFarmStateFile()
	deleteMonitorStateFile()

	log.Info("Farm successfully destroyed!")

	exit(0)
}

// runMonitoringLoop run loop which check farm status
func runMonitoringLoop(destroyAfter time.Time, maxWait int64) {
	destroyNotLater := time.Unix(destroyAfter.Unix()+maxWait, 0)

	if maxWait > 0 {
		log.Info(
			"Farm will be destroyed during the period %s - %s",
			timeutil.Format(destroyAfter, "%Y/%m/%d %H:%M:%S"),
			timeutil.Format(destroyNotLater, "%Y/%m/%d %H:%M:%S"),
		)
	} else {
		log.Info(
			"Farm will be destroyed after %s",
			timeutil.Format(destroyAfter, "%Y/%m/%d %H:%M:%S"),
		)
	}

	for {
		if !isTerrafarmActive() {
			log.Info("Farm destroyed manually. Shutdown monitor...")
			deleteMonitorStateFile()
			exit(0)
		}

		time.Sleep(time.Minute)

		if !isFarmMustBeDestroyed(destroyAfter, destroyNotLater) {
			continue
		}

		// Function return true if farm destroyed
		if destroyFarmByMonitor() {
			break
		}
	}
}

// destroyFarmByMonitor destroy farm
func destroyFarmByMonitor() bool {
	log.Info("Starting farm destroying...")

	prefs := findAndReadPreferences()
	vars, err := prefsToArgs(prefs, "-no-color", "-force")

	if err != nil {
		log.Error(err.Error())
		return false
	}

	templateDir := path.Join(getDataDir(), prefs.Template)

	fsutil.Push(templateDir)

	err = execTerraform(true, "destroy", vars)

	if err != nil {
		log.Error("Can't destroy farm - terrafarm return error: %v", err)
		return false
	}

	fsutil.Pop()

	return true
}

// isFarmMustBeDestroyed return true if farm must be destroyed
func isFarmMustBeDestroyed(destroyAfter, destroyNotLater time.Time) bool {
	now := time.Now().Unix()

	if now < destroyAfter.Unix() {
		return false
	}

	// MaxWait == 0
	if destroyAfter.Unix() == destroyNotLater.Unix() {
		return true
	}

	if now > destroyNotLater.Unix() {
		return true
	}

	farmState, err := readFarmState()

	if err != nil {
		log.Crit("Can't read farm state file: %v", err)
		exit(1)
	}

	activeBuildNodes := GetActiveBuildNodes(farmState.Preferences)

	if len(activeBuildNodes) == 0 {
		return true
	}

	log.Info(
		"%s still have active build processes, waiting...",
		strings.Join(activeBuildNodes, ", "),
	)

	return false
}

// getMonitorLogFilePath return path to monitor log file
func getMonitorLogFilePath() string {
	return path.Join(getDataDir(), MONITOR_LOG_FILE)
}

// getMonitorStateFilePath return path to monitor state file
func getMonitorStateFilePath() string {
	return path.Join(getDataDir(), MONITOR_STATE_FILE)
}

// deleteMonitorStateFile remote monitor state file
func deleteMonitorStateFile() error {
	return os.Remove(getMonitorStateFilePath())
}

// saveMonitorState save monitor state to file
func saveMonitorState(state *MonitorState) error {
	return jsonutil.EncodeToFile(getMonitorStateFilePath(), state)
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
func runMonitor(prefs *Preferences) error {
	monitorPrefs := fmt.Sprintf("%d", time.Now().Unix()+(prefs.TTL*60))

	if prefs.MaxWait > 0 {
		monitorPrefs += fmt.Sprintf("+%d", prefs.MaxWait*60)
	}

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("\n{s}EXEC → terrafarm --monitor %s{!}\n\n", monitorPrefs)
	}

	cmd := exec.Command("terrafarm", "--monitor", monitorPrefs)

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

// parseMonitoringPreferences parse monitoring preferences
func parseMonitoringPreferences(data string) (time.Time, int64, bool) {
	var (
		destroyAfter int64
		maxWait      int64
		err          error
	)

	if strings.Contains(data, "+") {
		dataSlice := strings.Split(data, "+")

		destroyAfter, err = strconv.ParseInt(dataSlice[0], 10, 64)

		if err != nil {
			return time.Time{}, 0, false
		}

		maxWait, err = strconv.ParseInt(dataSlice[1], 10, 64)

		if err != nil {
			return time.Time{}, 0, false
		}

		return time.Unix(destroyAfter, 0), maxWait, true
	}

	destroyAfter, err = strconv.ParseInt(data, 10, 64)

	if err != nil {
		return time.Time{}, 0, false
	}

	return time.Unix(destroyAfter, 0), 0, true
}

// ////////////////////////////////////////////////////////////////////////////////// //
