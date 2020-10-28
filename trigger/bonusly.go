package trigger

import (
	"github.com/evergreen-ci/evergreen"
	"github.com/evergreen-ci/evergreen/model/bonusly/bet"
	"github.com/evergreen-ci/evergreen/model/event"
	"github.com/evergreen-ci/evergreen/model/notification"
	"github.com/mongodb/grip/message"
	"github.com/pkg/errors"
)

const (
// kim: Notification templates
)

func init() {
	registry.registerEventHandler(event.ResourceTypeBonuslyBet, event.EventBonuslyBetUserMentioned, makeBonuslyBetTriggers)
}

type bonuslyBetTriggers struct {
	base

	event        *event.EventLogEntry
	data         *event.BonuslyBetEventData
	bet          *bet.Bet
	uiConfig     evergreen.UIConfig
	templateData bonuslyBetTemplateData
}

type bonuslyBetTemplateData struct {
	taskID         string
	versionID      string
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

	t.bet, err = bet.FindOne(bet.ByID(e.ResourceId))
	if err != nil {
		return errors.Wrap(err, "fetching Bonusly bet")
	}
	if t.bet == nil {
		return errors.New("could not find Bonusly bet")
	}

	if err = t.uiConfig.Get(evergreen.GetEnvironment()); err != nil {
		return errors.Wrap(err, "fetching UI config")
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
			Data: t.bet.ID,
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
	payload := t.makePayload(sub)
	if payload == nil {
		return nil, errors.Errorf("unsupported subscriber type: %s", sub.Subscriber.Type)
	}

	return notification.New(t.event.ID, sub.Trigger, &sub.Subscriber, payload)
}

func (t *bonuslyBetTriggers) makePayload(sub *event.Subscription) interface{} {
	switch sub.Subscriber.Type {
	case event.SlackSubscriberType:
		return t.slack()
	case event.EmailSubscriberType:
		return t.email()
	default:
		return nil
	}
}

func (t *bonuslyBetTriggers) slack() *notification.SlackPayload {
	// kim: TODO: implement template
	return nil
}

func (t *bonuslyBetTriggers) email() *message.Email {
	// kim: TODO: implement template
	return nil
}
