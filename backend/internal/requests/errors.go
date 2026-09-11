package requests

import "errors"

var (
	ErrArchiveMustBeZip = errors.New("архив должен иметь расширение zip")
	ErrArchiveTooLarge  = errors.New("файл слишком большой")
)
