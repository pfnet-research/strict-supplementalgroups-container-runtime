package runtime

type Interface interface {
	Exec(args []string) error
}
