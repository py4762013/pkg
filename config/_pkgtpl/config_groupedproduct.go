// +build ignore

package groupedproduct

import (
	"github.com/corestoreio/csfw/config/element"
	"github.com/corestoreio/csfw/store/scope"
)

// PackageConfiguration global configuration options for this package.
// Used in frontend and backend. See init() for details.
var PackageConfiguration element.SectionSlice

func init() {
	PackageConfiguration = element.MustNewConfiguration(
		&element.Section{
			ID:        "checkout",
			SortOrder: 305,
			Scope:     scope.PermAll,
			Groups: element.NewGroupSlice(
				&element.Group{
					ID:        "cart",
					SortOrder: 2,
					Scope:     scope.PermAll,
					Fields: element.NewFieldSlice(
						&element.Field{
							// Path: checkout/cart/grouped_product_image
							ID:        "grouped_product_image",
							Label:     `Grouped Product Image`,
							Type:      element.TypeSelect,
							SortOrder: 3,
							Visible:   element.VisibleYes,
							Scope:     scope.PermAll,
							Default:   `itself`,
							// SourceModel: Otnegam\Catalog\Model\Config\Source\Product\Thumbnail
						},
					),
				},
			),
		},
	)
}