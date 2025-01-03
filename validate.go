package structconf

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
)

func validate(configPointer any) error {
	configValidator := validator.New(validator.WithRequiredStructEnabled())
	err := configValidator.Struct(configPointer)
	if err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			errorMessage := &bytes.Buffer{}
			for _, fieldError := range validationErrors {
				validationTag := fieldError.Tag()
				if validationTag == "required" {
					errorMessage.WriteString(fmt.Sprintf("Missing required configuration: %s\n", fieldError.Namespace()))
				} else {
					errorMessage.WriteString(fmt.Sprintf("Configuration error: %s - %s\n", fieldError.StructField(), fieldError.ActualTag()))
				}
			}
			return errors.New(errorMessage.String())
		}
	}
	return nil
}
