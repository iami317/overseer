package main

import (
	"fmt"
	"github.com/menglh/overseer"
	"github.com/menglh/overseer/fetcher"
	"net/http"
	"os"
	"time"
)

//see example.sh for the use-case

// BuildID is compile-time variable
var BuildID = "1"

// convert your 'main()' into a 'prog(state)'
// 'prog()' is run in a child process
func prog(state overseer.State) {
	fmt.Printf("app#%s (%s) listening...\n", BuildID, state.ID)
	http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d, _ := time.ParseDuration(r.URL.Query().Get("d"))
		time.Sleep(d)
		fmt.Fprintf(w, "app#%s (%s) says hello\n", BuildID, state.ID)
	}))
	http.Serve(state.Listener, nil)
}

// then create another 'main' which runs the upgrades
// 'main()' is run in the initial process
func main() {
	fInfo, _ := os.Stat("./qd-bas-3")
	fmt.Println(fInfo.ModTime())
	overseer.Run(overseer.Config{
		Program: prog,
		Address: ":5001",
		Fetcher: &fetcher.HTTP{
			URL:      "https://192.168.8.208/api/v1/properties/agent_upgrade/linux",
			Interval: 10 * time.Second,
		},
		//&fetcher.File{Path: "my_app_next"},
		Debug: false, //display log of overseer actions
	})
}
