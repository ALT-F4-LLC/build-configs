package config

type Templater interface {
	Render() error
}
