package notimpl

type notImplementedError string

func (e notImplementedError) Error() string { return string(e) }

const ErrNotImplemented = notImplementedError("not implemented")
