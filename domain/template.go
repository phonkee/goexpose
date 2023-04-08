package domain

import "io"

type TemplateExecutor interface {
	Execute(wr io.Writer, data any) error
}
