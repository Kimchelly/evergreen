// Code generated by rest/model/codegen.go. DO NOT EDIT.

package model

import "github.com/evergreen-ci/evergreen/rest/model"

type APIMockEmbedded struct {
	One APIMockLayerOne `json:"one"`
}
type APIMockLayerOne struct {
	Two APIMockLayerTwo `json:"two"`
}
type APIMockLayerTwo struct {
	SomeField *string `json:"some_field"`
}

func APIMockEmbeddedBuildFromService(t model.MockEmbedded) *APIMockEmbedded {
	m := APIMockEmbedded{}
	m.One = *APIMockLayerOneBuildFromService(t.One)
	return &m
}

func APIMockEmbeddedToService(m APIMockEmbedded) *model.MockEmbedded {
	out := &model.MockEmbedded{}
	out.One = *APIMockLayerOneToService(m.One)
	return out
}

func APIMockLayerOneBuildFromService(t model.MockLayerOne) *APIMockLayerOne {
	m := APIMockLayerOne{}
	m.Two = *APIMockLayerTwoBuildFromService(t.Two)
	return &m
}

func APIMockLayerOneToService(m APIMockLayerOne) *model.MockLayerOne {
	out := &model.MockLayerOne{}
	out.Two = *APIMockLayerTwoToService(m.Two)
	return out
}

func APIMockLayerTwoBuildFromService(t model.MockLayerTwo) *APIMockLayerTwo {
	m := APIMockLayerTwo{}
	m.SomeField = StringPtrStringPtr(t.SomeField)
	return &m
}

func APIMockLayerTwoToService(m APIMockLayerTwo) *model.MockLayerTwo {
	out := &model.MockLayerTwo{}
	out.SomeField = StringPtrStringPtr(m.SomeField)
	return out
}
