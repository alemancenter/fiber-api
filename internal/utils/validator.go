package utils

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	_ = validate.RegisterValidation("password", validatePassword)
	_ = validate.RegisterValidation("phone", validatePhone)
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

// Validate validates a struct and returns a map of field errors
func Validate(s interface{}) map[string]string {
	err := validate.Struct(s)
	if err == nil {
		return nil
	}
	errors := make(map[string]string)
	for _, e := range err.(validator.ValidationErrors) {
		field := e.Field()
		errors[field] = translateError(e)
	}
	return errors
}

func translateError(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return "هذا الحقل مطلوب"
	case "email":
		return "البريد الإلكتروني غير صحيح"
	case "min":
		return "القيمة أقل من الحد الأدنى المسموح: " + e.Param()
	case "max":
		return "القيمة تتجاوز الحد الأقصى المسموح: " + e.Param()
	case "unique":
		return "هذه القيمة مستخدمة بالفعل"
	case "oneof":
		return "القيمة يجب أن تكون إحدى: " + e.Param()
	case "url":
		return "الرابط غير صحيح"
	case "password":
		return "كلمة المرور يجب أن تحتوي على 8 أحرف على الأقل"
	case "phone":
		return "رقم الهاتف غير صحيح"
	default:
		return "قيمة غير صحيحة"
	}
}

func validatePassword(fl validator.FieldLevel) bool {
	return len(fl.Field().String()) >= 8
}

func validatePhone(fl validator.FieldLevel) bool {
	re := regexp.MustCompile(`^\+?[0-9]{7,15}$`)
	return re.MatchString(fl.Field().String())
}

// SanitizeInput removes dangerous content from user input
func SanitizeInput(input string) string {
	// Strip HTML tags
	re := regexp.MustCompile(`<[^>]*>`)
	input = re.ReplaceAllString(input, "")

	// Strip dangerous SQL keywords (basic protection, GORM handles parameterization)
	dangerous := []string{
		"DROP TABLE", "DELETE FROM", "INSERT INTO", "UPDATE SET",
		"TRUNCATE", "ALTER TABLE", "CREATE TABLE", "--", "/*", "*/",
		"UNION SELECT", "OR 1=1", "AND 1=1",
	}
	upper := strings.ToUpper(input)
	for _, d := range dangerous {
		if strings.Contains(upper, d) {
			input = strings.ReplaceAll(input, d, "")
			input = strings.ReplaceAll(input, strings.ToLower(d), "")
		}
	}

	return strings.TrimSpace(input)
}

// SanitizeStruct sanitizes all string fields in a struct pointer
func SanitizeStruct(s interface{}) {
	v := reflect.ValueOf(s)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() == reflect.String && field.CanSet() {
			field.SetString(SanitizeInput(field.String()))
		} else if field.Kind() == reflect.Ptr && !field.IsNil() && field.Elem().Kind() == reflect.String && field.CanSet() {
			sanitized := SanitizeInput(field.Elem().String())
			field.Elem().SetString(sanitized)
		} else if field.Kind() == reflect.Struct {
			SanitizeStruct(field.Addr().Interface())
		}
	}
}
