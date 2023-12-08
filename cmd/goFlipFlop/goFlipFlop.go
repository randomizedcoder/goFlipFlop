package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/erikdubbelboer/gspt"
)

const (
	debugLevelCst = 11

	signalChannelSize = 10

	promListenCst           = ":9901"
	promPathCst             = "/metrics"
	promMaxRequestsInFlight = 10
	promEnableOpenMetrics   = true

	quantileError    = 0.05
	summaryVecMaxAge = 5 * time.Minute

	flipflopFrequencyCst = 1 * time.Minute
	divisorCst = 6

	interfaceCst = "eth0"

	scriptCst ="./configure_tc_qdisc_netem.bash"
	flipLatencyCst = "100ms"
	flopLatencyCst = "600ms"

	stopScriptCst = "./stop.bash"
	stoptimeCst = 10* time.Second

	showScriptCst = "./show.bash"

	timeoutCst = 2* time.Second

	uniqueStringCst = "flipper"

	goMaxProcsCst = 1
)

var (
	// Passed by "go build -ldflags" for the show version
	commit string
	date   string

	debugLevel int

	fullPaths map[string]string

	pC = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Subsystem: "counters",
			Name:      "goFlipFlop",
			Help:      "goFlipFlop counters",
		},
		[]string{"function", "variable", "type"},
	)
	pH = promauto.NewSummaryVec(
		prometheus.SummaryOpts{
			Subsystem: "histrograms",
			Name:      "goFlipFlop",
			Help:      "goFlipFlop historgrams",
			Objectives: map[float64]float64{
				0.1:  quantileError,
				0.5:  quantileError,
				0.99: quantileError,
			},
			MaxAge: summaryVecMaxAge,
		},
		[]string{"function", "variable", "type"},
	)
)

func main() {

	log.Println("goFlipFlop")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go initSignalHandler(cancel)

	version := flag.Bool("version", false, "version")

	// https://pkg.go.dev/net#Listen
	promListen := flag.String("promListen", promListenCst, "Prometheus http listening socket")
	promPath := flag.String("promPath", promPathCst, "Prometheus http path. Default = /metrics")
	// curl -s http://[::1]:9111/metrics 2>&1 | grep -v "#"
	// curl -s http://127.0.0.1:9111/metrics 2>&1 | grep -v "#"

	dl := flag.Int("dl", debugLevelCst, "nasty debugLevel")

	frequency := flag.Duration("frequency", flipflopFrequencyCst, "flip flop frequency")
	divisor := flag.Int("divisor",divisorCst,"Frequency divisor for updates")

	script := flag.String("conftc",scriptCst,"script file name")


	intf := flag.String("intf",interfaceCst,"interface argument to pass to the flip+flop scripts")
	flipLatency := flag.String("flipLatency",flipLatencyCst,"flipLatency")
	flopLatency := flag.String("flopLatency",flopLatencyCst,"flopLatency")

	stop := flag.String("stop",stopScriptCst,"stop script")
	stoptime := flag.Duration("stoptime",stoptimeCst,"stop time")

	show := flag.String("show",showScriptCst,"show script")

	timeout := flag.Duration("timeout",timeoutCst,"timeout running scripts")

	unique := flag.String("unique",uniqueStringCst,"Unique string for the process title")

	max := flag.Int("max",goMaxProcsCst,"GOMAXPROCS")

	flag.Parse()

	runtime.GOMAXPROCS(*max)

	if *version {
		fmt.Println("commit:", commit, "\tdate(UTC):", date)
		os.Exit(0)
	}

	debugLevel = *dl

	go initPromHandler(ctx, *promPath, *promListen)

	if debugLevel > 10 {
		log.Println("service init complete")
	}

	fullPaths = make(map[string]string)

	if !checkFilesExist([]string{*script,*stop,*show}) {
		log.Println("At least one file doesn't exists")
		os.Exit(1)
	}

	f := *frequency/ time.Duration(*divisor)
	t := time.NewTicker(f)

	latency := flopLatency
	for j:=0;;j++{

		pC.WithLabelValues("goFlipFlop", "for", "counter").Inc()

		for i:=0;i<*divisor;i++{

			pC.WithLabelValues("goFlipFlop", "i", "counter").Inc()

			<-t.C

			if debugLevel > 10 {
				//r := 1*time.Second
				//log.Println("tick","i:",i,"f:",f.Round(r).Seconds(),"i*f:",(time.Duration(i+1)*f).Round(r).Seconds(),"of:",(*frequency).Seconds())
				s := fmt.Sprintf("%s tick latency %s %d / %d ~= %.2f\n",*unique,*latency,i,*divisor,float64(i)/float64(*divisor))
				log.Print(s)
				gspt.SetProcTitle(s)
			}

			if i == 0 {

				pC.WithLabelValues("goFlipFlop", "flip_flop", "counter").Inc()

				if j>0 {
					if debugLevel > 10 {
						log.Println("sleeping:",*stoptime)
					}
					s := time.NewTimer(*stoptime)
					runCommandwithTimeout(*stop, *timeout,false,[]string{*intf})
					if debugLevel > 10 {
						runCommandwithTimeout(*show, *timeout,true,[]string{*intf})
					}
					<-s.C
					if debugLevel > 100 {
						log.Println("awoke")
					}
				}

				if latency == flopLatency {
					latency = flipLatency
					pC.WithLabelValues("goFlipFlop", "flip", "counter").Inc()
				} else {
					latency = flopLatency
					pC.WithLabelValues("goFlipFlop", "flop", "counter").Inc()
				}
				if debugLevel > 10 {
					log.Println("latency:",*latency)
				}

				runCommandwithTimeout(*script, *timeout, false,[]string{*intf, *latency})
				if debugLevel > 10 {
					runCommandwithTimeout(*show, *timeout,true,[]string{*intf})
				}

			}

		}
	}

	//log.Println("goFlipFlop: That's all Folks!")
}

// initSignalHandler sets up signal handling for the process, and
// will call cancel() when recieved
func initSignalHandler(cancel context.CancelFunc) {
	c := make(chan os.Signal, signalChannelSize)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	<-c
	log.Printf("Signal caught, closing application")
	cancel()
	os.Exit(0)
}

// initPromHandler starts the prom handler with error checking
func initPromHandler(ctx context.Context, promPath string, promListen string) {
	// https: //pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp?tab=doc#HandlerOpts
	http.Handle(promPath, promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			EnableOpenMetrics:   promEnableOpenMetrics,
			MaxRequestsInFlight: promMaxRequestsInFlight,
		},
	))
	go func() {
		err := http.ListenAndServe(promListen, nil)
		if err != nil {
			log.Fatal("prometheus error", err)
		}
	}()
}


func runCommandwithTimeout(command string, timeout time.Duration, show bool,args []string){

	startTime:=time.Now()
	defer func(){pH.WithLabelValues("runCommandwithTimeout", command, "counter").Observe(float64(time.Since(startTime).Seconds()))}()
	pC.WithLabelValues("runCommandwithTimeout", "start", "counter").Inc()
	pC.WithLabelValues("runCommandwithTimeout", command, "counter").Inc()


	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fullCommand := getFullCommand(command)

	if debugLevel > 10 {
		log.Println("runCommandwithTimeout command:",command,"fullCommand:",fullCommand,"args:",args)
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	cmd := exec.CommandContext(ctx, fullCommand, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	if err != nil {
		if debugLevel > 10 {
			log.Println("cmd err:",err)
			log.Println("cmd stdout:",stdout)
			log.Println("cmd stderr:",stderr)
		}
	}

	if show  {
		log.Print("cmd stdout:",stdout)
		log.Print("cmd stderr:",stderr)
	}
}


// getFullCommand does the expensive lookup to find the binary command
// in the path, and then caches this into the fullPaths map
// this way, we only do the expensive lookup once per invocation of this program
func getFullCommand(command string) (fullCommand string) {
	pC.WithLabelValues("getFullCommand", "start", "counter").Inc()
	var ok bool
	fullCommand, ok = fullPaths[command]
	if !ok {
		pC.WithLabelValues("getFullCommand", "LookPath", "counter").Inc()
		var err error
		fullCommand, err = exec.LookPath(command)
		if err != nil {
			log.Fatal("can't find command", err)
		}
		fullPaths[command] = fullCommand
	}
	return fullCommand
}

func checkFilesExist(files []string)(exists bool) {
	for _,f := range files{
		if !checkFileExists(f) {
			if debugLevel > 10 {
				log.Println("file doesn't exist:",f)
			}
			return
		}
	}
	exists = true
	return exists
}

func checkFileExists(file string) bool {
	_, error := os.Stat(file)
	return !errors.Is(error, os.ErrNotExist)
}