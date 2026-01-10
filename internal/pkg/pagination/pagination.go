package pagination

import (
	"math"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"github.com/kkz6/launch-go/internal/pkg/response"
)

type Params struct {
	Page    int
	PerPage int
}

func GetParams(c *fiber.Ctx) Params {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "15"))

	if page < 1 {
		page = 1
	}

	if perPage < 1 {
		perPage = 15
	}

	if perPage > 100 {
		perPage = 100
	}

	return Params{
		Page:    page,
		PerPage: perPage,
	}
}

func (p Params) Offset() int {
	return (p.Page - 1) * p.PerPage
}

func Paginate[T any](db *gorm.DB, params Params, dest *[]T) (*response.Meta, error) {
	var total int64

	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	if err := db.Offset(params.Offset()).Limit(params.PerPage).Find(dest).Error; err != nil {
		return nil, err
	}

	lastPage := int(math.Ceil(float64(total) / float64(params.PerPage)))

	return &response.Meta{
		CurrentPage: params.Page,
		PerPage:     params.PerPage,
		Total:       total,
		LastPage:    lastPage,
	}, nil
}
