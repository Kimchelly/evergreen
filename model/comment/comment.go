package comment

import (
	"time"

	"github.com/evergreen-ci/evergreen/db"
)

type Comment struct {
	Id           string    `bson:"_id" json:"_id"`
	CreateTime   time.Time `bson:"create_time" json:"create_time,omitempty"`
	ThreadId     string    `bson:"thread_id" json:"thread_id"`
	ResourceType string    `bson:"resource_type json:"resource_type"`
	ResourceId   string    `bson:"resource_id" json:"resource_id"`
	UserId       string    `bson:"user_id json:"user_id"`
	Message      string    `bson:"message" json:"message"`
	Likes        []string  `bson:"likes json:"likes"`
	Dislikes     []string  `bson:"dislikes" json:"dislikes"`
}

func FindByResourceAndType(resourceType string, resourceId string) ([]Comment, error) {
	return Find(ByResourceTypeAndResourceId(resourceType, resourceId))
}

// Insert writes the b to the db.
func (c *Comment) Insert() error {
	return db.Insert(Collection, c)
}
