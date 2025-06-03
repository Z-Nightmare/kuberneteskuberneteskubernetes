package translate

import "zeusro.com/hermes/model"

type Translator interface {
	Translate(source string, country string) (target model.Language, err error)
}
