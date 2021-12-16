package model

import (
	"fmt"

	"github.com/robertkrimen/otto"
)

type ExecutableStringOrNull struct {
	Valid  bool
	String string
}

func (e *ExecutableStringOrNull) UnmarshalYAML(unmarshal func(interface{}) error) error {
	str := ""
	err := unmarshal(&str)
	if err != nil {
		return err
	}

	// make sure to dereference before assignment,
	// otherwise only the local variable will be overwritten
	// and not the value the pointer actually points to
	*e = ExecutableStringOrNull{}
	if str == "" {
		e.Valid = false
		e.String = ""

		return nil
	}

	e.String = str
	e.Valid = true

	return nil
}

func (e *ExecutableStringOrNull) Execute(vm *otto.Otto) (otto.Value, error) {
	if !e.Valid {
		return otto.NullValue(), nil
	}

	mutexVm.Lock()
	defer mutexVm.Unlock()
	val, err := vm.Run(e.String)
	if err != nil {
		return otto.NullValue(), fmt.Errorf("can't execute script: %s", err)
	}

	return val, nil
}
