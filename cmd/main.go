package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/sapcc/helm-realitycheck/pkg/realitycheck"
	"github.com/spf13/pflag"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func main() {

	if f := flag.Lookup("logtostderr"); f != nil {
		f.Value.Set("true")
	}
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	getter := genericclioptions.NewConfigFlags()

	getter.AddFlags(pflag.CommandLine)

	pflag.Parse()

	sigs := make(chan os.Signal, 1)
	stop := make(chan struct{})
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM) // Push signals into channel

	go func() {
		<-sigs
		glog.Info("Shutdown signal caught!")
		close(stop)
	}()

	if err := realitycheck.New(getter).Run(stop); err != nil {
		glog.Fatal(err)
	}

}
