package trigger

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/model/bonusly/betpool"
	"github.com/evergreen-ci/evergreen/model/event"
	"github.com/evergreen-ci/evergreen/model/notification"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
	// kim: Notification templates
	bonuslyBetMentionSlackBody            = `You have been invited by {{.User}} to a Bonusly bet! Visit <{{.URL}}|this link> to place a bet.`
	bonuslyBetMentionSlackAttachmentTitle = "Bonusly Bet"
)

func init() {
	registry.registerEventHandler(event.ResourceTypeBonuslyBet, event.EventBonuslyBetUserMentioned, makeBonuslyBetTriggers)
}

type bonuslyBetTriggers struct {
	base

	event        *event.EventLogEntry
	data         *event.BonuslyBetEventData
	bp           *betpool.BettingPool
	uiConfig     evergreen.UIConfig
	templateData bonuslyBetTemplateData
}

type bonuslyBetTemplateData struct {
	mentioningUser string
	url            string
}

func (t *bonuslyBetTriggers) Fetch(e *event.EventLogEntry) error {
	var ok bool
	var err error
	t.data, ok = e.Data.(*event.BonuslyBetEventData)
	if !ok {
		return errors.Errorf("expected Bonusly bet event data, got %T", e.Data)
	}

	t.bp, err = betpool.FindOne(betpool.ByID(e.ResourceId))
	if err != nil {
		return errors.Wrap(err, "fetching Bonusly betting pool")
	}
	if t.bp == nil {
		return errors.New("could not find Bonusly betting pool")
	}

	if err = t.uiConfig.Get(evergreen.GetEnvironment()); err != nil {
		return errors.Wrap(err, "fetching UI config")
	}

	var url string
	if t.bp.TaskID != "" {
		url = fmt.Sprintf("%s/task/%s", t.uiConfig.Url, t.bp.TaskID)
	}
	if t.bp.VersionID != "" {
		url = fmt.Sprintf("%s/version/%s", t.uiConfig.Url, t.bp.VersionID)
	}
	t.templateData = bonuslyBetTemplateData{
		mentioningUser: t.data.User,
		url:            url,
	}

	return nil
}

// kim: TODO: figure out what this selector thing does, which I think has to do
// with DB querying to filter which events apply to this trigger, but not sure.
func (t *bonuslyBetTriggers) Selectors() []event.Selector {
	// kim: TODO: unsure if these selectors are correctly, especially the
	// owner/mentioned selector.
	return []event.Selector{
		{
			Type: event.SelectorID,
			Data: t.bp.ID,
		},
		{
			Type: event.SelectorObject,
			Data: event.ObjectBonuslyBet,
		},
		{
			Type: event.SelectorOwner,
			Data: t.data.MentionedUser,
		},
	}
}

func makeBonuslyBetTriggers() eventHandler {
	t := &bonuslyBetTriggers{}
	t.base.triggers = map[string]trigger{
		event.TriggerMentioned: t.userMentioned,
	}
	return t
}

func (t *bonuslyBetTriggers) userMentioned(sub *event.Subscription) (*notification.Notification, error) {
	return t.generate(sub)
}

func (t *bonuslyBetTriggers) generate(sub *event.Subscription) (*notification.Notification, error) {
	payload, err := t.makePayload(sub)
	if err != nil {
		return nil, errors.Wrap(err, "creating Bonusly bet payload")
	}

	return notification.New(t.event.ID, sub.Trigger, &sub.Subscriber, payload)
}

func (t *bonuslyBetTriggers) makePayload(sub *event.Subscription) (interface{}, error) {
	switch sub.Subscriber.Type {
	case event.SlackSubscriberType:
		return t.slack()
	case event.EmailSubscriberType:
		return t.email()
	default:
		return nil, errors.Errorf("unsupported subscriber type %T", sub.Subscriber.Type)
	}
}

func (t *bonuslyBetTriggers) slack() (*notification.SlackPayload, error) {
	attachment := message.SlackAttachment{
		Title:     bonuslyBetMentionSlackAttachmentTitle,
		TitleLink: t.templateData.url,
		Color:     evergreenSuccessColor,
	}

	buf := &bytes.Buffer{}
	tmpl, err := template.New("subject").Parse(bonuslyBetMentionSlackBody)
	if err != nil {
		return nil, errors.Wrap(err, "parsing body template")
	}
	if err = tmpl.Execute(buf, t); err != nil {
		return nil, errors.Wrap(err, "executing body template")
	}

	return &notification.SlackPayload{
		Body:        buf.String(),
		Attachments: []message.SlackAttachment{attachment},
	}, nil
}

func (t *bonuslyBetTriggers) email() (*message.Email, error) {
	// kim: TODO: implement template
	return nil, errors.New("email notifications not supported for Bonusly bet notifications")
}
