package buildkite

import (
	"fmt"
	"github.com/buildkite/agent/buildkite/logger"
	"os"
	"time"
)

type Agent struct {
	// The name of the new agent
	Name string `json:"name"`

	// The access token for the agent
	AccessToken string `json:"access_token"`

	// Hostname of the machine
	Hostname string `json:"hostname"`

	// Operating system for this machine
	OS string `json:"os"`

	// If this agent is allowed to perform command evaluation
	CommandEval bool `json:"script_eval_enabled"`

	// The priority of the agent
	Priority string `json:"priority,omitempty"`

	// The version of this agent
	Version string `json:"version"`

	// Meta data for the agent
	MetaData []string `json:"meta_data"`

	// The PID of the agent
	PID int `json:"pid,omitempty"`

	// The client the agent will use to communicate to the API
	Client Client

	// The boostrap script to run
	BootstrapScript string

	// The path to the run the builds in
	BuildPath string

	// Where bootstrap hooks are found
	HooksPath string

	// Whether or not the agent is allowed to automatically accept SSH
	// fingerprints
	AutoSSHFingerprintVerification bool

	// Run jobs in a PTY
	RunInPty bool

	// The currently running Job
	Job *Job

	// This is true if the agent should no longer accept work
	stopping bool
}

func (c *Client) AgentRegister(agent *Agent) error {
	return c.Post(&agent, "/register", agent)
}

func (c *Client) AgentConnect(agent *Agent) error {
	return c.Post(&agent, "/connect", agent)
}

func (c *Client) AgentDisconnect(agent *Agent) error {
	return c.Post(&agent, "/disconnect", agent)
}

func (a *Agent) String() string {
	return fmt.Sprintf("Agent{Name: %s, Hostname: %s, PID: %d, RunInPty: %t}", a.Name, a.Hostname, a.PID, a.RunInPty)
}

func (a *Agent) Start() {
	// How long the agent will wait when no jobs can be found.
	idleSeconds := 5
	sleepTime := time.Duration(idleSeconds*1000) * time.Millisecond

	for {
		// Did the agent try and stop during the last job run?
		if a.stopping {
			a.Stop()
		} else {
			a.Ping()
		}

		// Sleep for a while before we check again
		time.Sleep(sleepTime)
	}
}

func (a *Agent) Ping() {
	ping, err := a.Client.AgentPing()
	if err != nil {
		logger.Warn("Failed to ping (%s)", err)
		return
	}

	// Is there a message that should be shown in the logs?
	if ping.Message != "" {
		logger.Info(ping.Message)
	}

	// Should the agent disconnect?
	if ping.Action == "disconnect" {
		a.Stop()
		return
	}

	// Do nothing if there's no job
	if ping.Job == nil {
		return
	}

	logger.Info("Assigned job %s. Accepting...", ping.Job.ID)

	// Accept the job
	job, err := a.Client.JobAccept(ping.Job)
	if err != nil {
		logger.Error("Failed to accept the job (%s)", err)
		return
	}

	// Confirm that it's been accepted
	if job.State != "accepted" {
		logger.Error("Can not accept job with state `%s`", job.State)
		return
	}

	a.Job = job
	job.Run(a)
	a.Job = nil
}

func (a *Agent) Stop() {
	// Disconnect from Buildkite
	logger.Info("Disconnecting...")
	a.Client.AgentDisconnect(a)

	// Kill the process
	os.Exit(0)
}
