package client

import (
	"fmt"
	"strings"
)

type param struct {
	Name  string
	Value string
}

func verifyParams(params ...param) error {
	var missing []string
	for _, p := range params {
		if strings.TrimSpace(p.Value) == "" {
			missing = append(missing, p.Name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required parameter(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

type replicaError struct {
	Address string
	Message string
}

func newReplicaError(address string, err error) replicaError {
	return replicaError{Address: address, Message: err.Error()}
}

func (e replicaError) Error() string {
	return fmt.Sprintf("%v: %v", e.Address, e.Message)
}

type taskError struct {
	ReplicaErrors []replicaError
}

func newTaskError(errors ...replicaError) *taskError {
	return &taskError{ReplicaErrors: append([]replicaError{}, errors...)}
}

func (e *taskError) Error() string {
	var messages []string
	for _, replicaErr := range e.ReplicaErrors {
		messages = append(messages, replicaErr.Error())
	}
	if len(messages) == 0 {
		return "Unknown"
	}
	return strings.Join(messages, "; ")
}

func (e *taskError) Append(replicaErr replicaError) {
	e.ReplicaErrors = append(e.ReplicaErrors, replicaErr)
}
