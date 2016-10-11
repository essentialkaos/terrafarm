package cli

// ////////////////////////////////////////////////////////////////////////////////// //
//                                                                                    //
//                     Copyright (c) 2009-2016 Essential Kaos                         //
//      Essential Kaos Open Source License <http://essentialkaos.com/ekol?en>         //
//                                                                                    //
// ////////////////////////////////////////////////////////////////////////////////// //

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"pkg.re/essentialkaos/ek.v5/arg"
	"pkg.re/essentialkaos/ek.v5/fmtc"
	"pkg.re/essentialkaos/ek.v5/fsutil"
	"pkg.re/essentialkaos/ek.v5/jsonutil"
	"pkg.re/essentialkaos/ek.v5/log"
	"pkg.re/essentialkaos/ek.v5/path"
	"pkg.re/essentialkaos/ek.v5/signal"
	"pkg.re/essentialkaos/ek.v5/timeutil"
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

// startFarmMonitor starts monitoring process
func startFarmMonitor() {
	log.Set(getMonitorLogFilePath(), 0644)

	log.Aux(SEPARATOR)
	log.Aux("Terrafarm %s monitor started", VER)

	state, err := getMonitorState()

	if err != nil {
		log.Crit(err.Error())
		exit(1)
	}

	signal.Handlers{
		signal.USR1: usr1SignalHandler,
		signal.TERM: termSignalHandler,
	}.TrackAsync()

	runMonitoringLoop(time.Unix(state.DestroyAfter, 0), state.MaxWait)

	deleteFarmStateFile()
	deleteMonitorStateFile()

	log.Info("Farm successfully destroyed!")

	exit(0)
}

// getMonitorState return monitor state
func getMonitorState() (*MonitorState, error) {
	var (
		state *MonitorState
		err   error
	)

	if arg.GetS(ARG_MONITOR) == "-1" {
		if !fsutil.IsExist(getMonitorStateFilePath()) {
			return nil, fmtc.Errorf("Can't start monitoring process: state file not exist")
		}

		state, err = readMonitorState()
	}

	if state == nil {
		destroyAfter, maxWait, ok := parseMonitoringPreferences(arg.GetS(ARG_MONITOR))

		if !ok {
			return nil, fmtc.Errorf("Can't parse given monitor preferences (%s)", arg.GetS(ARG_MONITOR))
		}

		state = &MonitorState{
			Started:      time.Now().Unix(),
			DestroyAfter: destroyAfter.Unix(),
			MaxWait:      maxWait,
		}
	}

	state.Pid = os.Getpid()

	err = saveMonitorState(state)

	if err != nil {
		return nil, fmtc.Errorf("Can't save monitor state to file: %v", err)
	}

	return state, nil
}

// killMonitorProcess kill monitor process
func killMonitorProcess() error {
	state, err := readMonitorState()

	if err != nil {
		return err
	}

	if !fsutil.IsExist(fmtc.Sprintf("/proc/%d", state.Pid)) {
		return fmtc.Errorf("Monitor process with pid %d is not found", state.Pid)
	}

	err = signal.Send(state.Pid, signal.USR1)

	if err != nil {
		return err
	}

	return nil
}

// usr1SignalHandler is USR1 signal handler
func usr1SignalHandler() {
	log.Info("Got USR1 signal, restarting...")
	exit(0)
}

// termSignalHandler is TERM signal handler
func termSignalHandler() {
	log.Info("Got TERM signal, shutdown...")
	deleteMonitorStateFile()
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

	priceMessage, priceMessageComment := getUsagePriceMessage()

	if priceMessage != "" {
		log.Info("Usage price: %s (%s)", priceMessage, priceMessageComment)
	}

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

	activeBuildNodes := getActiveBuildNodesNames(farmState.Preferences)

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
		return nil, fmtc.Errorf("Monitor state file is not exist")
	}

	err := jsonutil.DecodeFile(stateFile, state)

	if err != nil {
		return nil, err
	}

	return state, nil
}

// startMonitorProcess start or restart monitoring process
func startMonitorProcess(prefs *Preferences, restart bool) error {
	monitorPrefs := "-1"

	if !restart {
		monitorPrefs = fmtc.Sprintf("%d", time.Now().Unix()+(prefs.TTL*60))

		if prefs.MaxWait > 0 {
			monitorPrefs += fmtc.Sprintf("+%d", prefs.MaxWait*60)
		}
	}

	if arg.GetB(ARG_DEBUG) {
		fmtc.Printf("\n{s-}EXEC → terrafarm --monitor %s{!}\n\n", monitorPrefs)
	}

	cmd := exec.Command("terrafarm", "--monitor", monitorPrefs)
	err := cmd.Start()

	if err != nil {
		return err
	}

	// 0.125 * 40 = 5 sec
	for i := 0; i < 40; i++ {
		if isMonitorActive() {
			return nil
		}

		time.Sleep(125 * time.Millisecond)
	}

	return fmtc.Errorf("Monitor does not start more than 5 seconds")
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
