package plugin

import (
	"fmt"
	"html/template"
	"net/http"
	"testing"

	"github.com/pkg/errors"
	. "github.com/smartystreets/goconvey/convey"
)

// ===== Mock UI Plugin =====

// simple plugin type that has a name and ui config
type MockPlugin struct {
	NickName string
	Conf     *PanelConfig
}

func (mp *MockPlugin) Name() string {
	return mp.NickName
}

func (mp *MockPlugin) GetUIHandler() http.Handler {
	return nil
}

func (mp *MockPlugin) Configure(conf map[string]any) error {
	return nil
}

func (mp *MockPlugin) GetPanelConfig() (*PanelConfig, error) {
	return mp.Conf, nil
}

// ===== Tests =====

func TestPanelManagerRegistration(t *testing.T) {
	var ppm PanelManager
	Convey("With a simple plugin panel manager", t, func() {
		ppm = &SimplePanelManager{}

		Convey("and a registered set of test plugins without panels", func() {
			uselessPlugins := []Plugin{
				&MockPlugin{
					NickName: "no_ui_config",
					Conf:     nil,
				},
				&MockPlugin{
					NickName: "config_with_no_panels",
					Conf:     &PanelConfig{},
				},
			}
			err := ppm.RegisterPlugins(uselessPlugins)
			So(err, ShouldBeNil)

			Convey("no ui panel data should be returned for any scope", func() {
				data, err := ppm.UIData(UIContext{}, TaskPage)
				So(err, ShouldBeNil)
				So(data["no_ui_config"], ShouldBeNil)
				So(data["config_with_no_panels"], ShouldBeNil)
				data, err = ppm.UIData(UIContext{}, BuildPage)
				So(err, ShouldBeNil)
				So(data["no_ui_config"], ShouldBeNil)
				So(data["config_with_no_panels"], ShouldBeNil)
				data, err = ppm.UIData(UIContext{}, VersionPage)
				So(err, ShouldBeNil)
				So(data["no_ui_config"], ShouldBeNil)
				So(data["config_with_no_panels"], ShouldBeNil)
			})
		})

		Convey("registering a plugin panel with no page should fail", func() {
			badPanelPlugins := []Plugin{
				&MockPlugin{
					NickName: "bad_panel",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{PanelHTML: "<marquee> PANEL </marquee>"},
						},
					},
				},
			}
			err := ppm.RegisterPlugins(badPanelPlugins)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "Page")
		})

		Convey("registering the same plugin name twice should fail", func() {
			conflictingPlugins := []Plugin{
				&MockPlugin{
					NickName: "a",
					Conf:     nil,
				},
				&MockPlugin{
					NickName: "a",
					Conf:     &PanelConfig{},
				},
			}
			err := ppm.RegisterPlugins(conflictingPlugins)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "already")
		})

		Convey("registering more than one data function to the same page "+
			"for the same plugin should fail", func() {
			dataPlugins := []Plugin{
				&MockPlugin{
					NickName: "data_function_fan",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page: TaskPage,
								DataFunc: func(context UIContext) (any, error) {
									return 100, nil
								}},
							{
								Page: TaskPage,
								DataFunc: func(context UIContext) (any, error) {
									return nil, errors.New("this function just errors")
								}},
						},
					},
				},
			}
			err := ppm.RegisterPlugins(dataPlugins)
			So(err, ShouldNotBeNil)
			So(err.Error(), ShouldContainSubstring, "function is already registered")
		})
	})
}

func TestPanelManagerRetrieval(t *testing.T) {
	var ppm PanelManager

	Convey("With a simple plugin panel manager", t, func() {
		ppm = &SimplePanelManager{}

		Convey("and a registered set of test plugins with panels", func() {
			// These 3 plugins exist to check the sort output of the manager.
			// For consistency, plugin panels and includes are ordered by plugin name
			// and then by the order of their declaration in the Panels array.
			// This test asserts that the panels in A come before B which come
			// before C, even though they are not in the plugin array in that order.
			testPlugins := []Plugin{
				&MockPlugin{
					NickName: "A_the_first_letter",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page:     TaskPage,
								Position: PageCenter,
								Includes: []template.HTML{
									"0",
									"1",
								},
								PanelHTML: "0",
								DataFunc: func(context UIContext) (any, error) {
									return 1000, nil
								},
							},
							{
								Page:     TaskPage,
								Position: PageCenter,
								Includes: []template.HTML{
									"2",
									"3",
								},
								PanelHTML: "1",
							},
							{
								Page:     TaskPage,
								Position: PageLeft,
								Includes: []template.HTML{
									"4",
								},
								PanelHTML: "X",
							},
						},
					},
				},
				&MockPlugin{
					NickName: "C_the_third_letter",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page:     TaskPage,
								Position: PageCenter,
								Includes: []template.HTML{
									"7",
									"8",
								},
								PanelHTML: "3",
								DataFunc: func(context UIContext) (any, error) {
									return 2112, nil
								},
							},
						},
					},
				},
				&MockPlugin{
					NickName: "B_the_middle_letter",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page:     TaskPage,
								Position: PageCenter,
								Includes: []template.HTML{
									"5",
								},
								PanelHTML: "2",
								DataFunc: func(context UIContext) (any, error) {
									return 1776, nil
								},
							},
							{
								Page:     TaskPage,
								Position: PageLeft,
								Includes: []template.HTML{
									"6",
								},
								PanelHTML: "Z",
							},
						},
					},
				},
			}

			err := ppm.RegisterPlugins(testPlugins)
			So(err, ShouldBeNil)

			Convey("retrieved includes for the task page should be in correct "+
				"stable alphabetical order by plugin name", func() {
				includes, err := ppm.Includes(TaskPage)
				So(err, ShouldBeNil)
				So(includes, ShouldNotBeNil)

				// includes == [0 1 2 ... ]
				for i := 1; i < len(includes); i++ {
					So(includes[i], ShouldBeGreaterThan, includes[i-1])
				}
			})
			Convey("retrieved panel HTML for the task page should be in correct "+
				"stable alphabetical order by plugin name", func() {
				panels, err := ppm.Panels(TaskPage)
				So(err, ShouldBeNil)
				So(len(panels.Right), ShouldEqual, 0)
				So(len(panels.Left), ShouldBeGreaterThan, 0)
				So(len(panels.Center), ShouldBeGreaterThan, 0)

				// left == [X Z]
				So(panels.Left[0], ShouldBeLessThan, panels.Left[1])

				// center == [0 1 2 3]
				for i := 1; i < len(panels.Center); i++ {
					So(panels.Center[i], ShouldBeGreaterThan, panels.Center[i-1])
				}
			})
			Convey("data functions populate the results map with their return values", func() {
				uiData, err := ppm.UIData(UIContext{}, TaskPage)
				So(err, ShouldBeNil)
				So(len(uiData), ShouldBeGreaterThan, 0)
				So(uiData["A_the_first_letter"], ShouldEqual, 1000)
				So(uiData["B_the_middle_letter"], ShouldEqual, 1776)
				So(uiData["C_the_third_letter"], ShouldEqual, 2112)
			})
		})
	})
}

func TestPluginUIDataFunctionErrorHandling(t *testing.T) {
	var ppm PanelManager

	Convey("With a simple plugin panel manager", t, func() {
		ppm = &SimplePanelManager{}

		Convey("and a set of plugins, some with erroring data functions", func() {
			errorPlugins := []Plugin{
				&MockPlugin{
					NickName: "error1",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page:     TaskPage,
								Position: PageCenter,
								DataFunc: func(context UIContext) (any, error) {
									return nil, errors.New("Error #1")
								},
							},
						},
					},
				},
				&MockPlugin{
					NickName: "error2",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page:     TaskPage,
								Position: PageCenter,
								DataFunc: func(context UIContext) (any, error) {
									return nil, errors.New("Error #2")
								},
							},
						},
					},
				},
				&MockPlugin{
					NickName: "error3 not found",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page:     TaskPage,
								Position: PageCenter,
								DataFunc: func(_ UIContext) (any, error) {
									return nil, errors.New("Error")
								},
							},
						},
					},
				},
				&MockPlugin{
					NickName: "good",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page:     TaskPage,
								Position: PageCenter,
								DataFunc: func(_ UIContext) (any, error) {
									return "fine", nil
								},
							},
						},
					},
				},
			}
			err := ppm.RegisterPlugins(errorPlugins)
			So(err, ShouldBeNil)
			data, err := ppm.UIData(UIContext{}, TaskPage)
			So(err, ShouldNotBeNil)

			Convey("non-broken functions should succeed", func() {
				So(data["good"], ShouldEqual, "fine")
			})

			Convey("and reasonable error messages should be produced for failures", func() {
				So(err.Error(), ShouldContainSubstring, "error1")
				So(err.Error(), ShouldContainSubstring, "Error #1")
				So(err.Error(), ShouldContainSubstring, "error2")
				So(err.Error(), ShouldContainSubstring, "Error #2")
				So(err.Error(), ShouldContainSubstring, "error3")
			})
		})
		Convey("and a plugin that panics", func() {
			errorPlugins := []Plugin{
				&MockPlugin{
					NickName: "busted",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page:     TaskPage,
								Position: PageCenter,
								DataFunc: func(_ UIContext) (any, error) {
									panic("BOOM")
								},
							},
						},
					},
				},
				&MockPlugin{
					NickName: "good",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page:     TaskPage,
								Position: PageCenter,
								DataFunc: func(_ UIContext) (any, error) {
									return "still fine", nil
								},
							},
						},
					},
				},
			}

			Convey("reasonable error messages should be produced", func() {
				err := ppm.RegisterPlugins(errorPlugins)
				So(err, ShouldBeNil)
				data, err := ppm.UIData(UIContext{}, TaskPage)
				So(err, ShouldNotBeNil)
				So(data["good"], ShouldEqual, "still fine")
				So(err.Error(), ShouldContainSubstring, "panic")
				So(err.Error(), ShouldContainSubstring, "BOOM")
				So(err.Error(), ShouldContainSubstring, "busted")
			})
		})
	})
}

func TestUIDataInjection(t *testing.T) {
	var ppm PanelManager

	Convey("With a simple plugin panel manager", t, func() {
		ppm = &SimplePanelManager{}

		Convey("and a registered set of test plugins with injection needs", func() {
			funcPlugins := []Plugin{
				&MockPlugin{
					NickName: "combine",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page:     TaskPage,
								Position: PageCenter,
								DataFunc: func(ctx UIContext) (any, error) {
									return ctx.Task.Id + ctx.Build.Id + ctx.Version.Id, nil
								},
							},
						},
					},
				},
				&MockPlugin{
					NickName: "userhttpapiserver",
					Conf: &PanelConfig{
						Panels: []UIPanel{
							{
								Page:     TaskPage,
								Position: PageCenter,
								DataFunc: func(ctx UIContext) (any, error) {
									return fmt.Sprintf("%v.%v@%v", ctx.User.Email(), ctx.Settings.Api.URL, nil), nil
								},
							},
						},
					},
				},
			}
			err := ppm.RegisterPlugins(funcPlugins)
			So(err, ShouldBeNil)
		})
	})
}
