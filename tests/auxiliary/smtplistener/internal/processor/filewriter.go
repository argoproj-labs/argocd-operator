package processor

import (
	"fmt"
	"io/ioutil"

	"github.com/flashmob/go-guerrilla/backends"
	"github.com/flashmob/go-guerrilla/mail"
)

var FileWriter = func() backends.Decorator {
	initializer := backends.InitializeWith(func(backendConfig backends.BackendConfig) error {
		return nil
	})
	// register our initializer
	backends.Svc.AddInitializer(initializer)
	// When shutting down
	backends.Svc.AddShutdowner(backends.ShutdownWith(func() error {
		return nil
	}))

	return func(p backends.Processor) backends.Processor {
		return backends.ProcessWith(func(e *mail.Envelope, task backends.SelectTask) (backends.Result, error) {
			if task == backends.TaskSaveMail {
				var stringer fmt.Stringer
				stringer = e
				data := []byte(stringer.String())
				key := fmt.Sprintf("%s%s", "/tmp/", e.QueuedId)
				err := ioutil.WriteFile(key, data, 0666)
				fmt.Println(err)
				return p.Process(e, task)
			} else {
				return p.Process(e, task)
			}

		})
	}
}
