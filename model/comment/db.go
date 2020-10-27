package comment

import (
	"github.com/evergreen-ci/evergreen/db"
	"github.com/mongodb/anser/bsonutil"
	adb "github.com/mongodb/anser/db"
	"go.mongodb.org/mongo-driver/bson"
)

// The MongoDB collection for comment documents.
const Collection = "comments"

var (
	// bson fields for the comment struct
	IdKey           = bsonutil.MustHaveTag(Comment{}, "Id")
	CreateTimeKey   = bsonutil.MustHaveTag(Comment{}, "CreateTime")
	ResourceTypeKey = bsonutil.MustHaveTag(Comment{}, "ResourceType")
	ResourceIdKey   = bsonutil.MustHaveTag(Comment{}, "ResourceId")
)

func ByResourceTypeAndResourceId(resourceType string, resourceId string) db.Q {
	return db.Query(bson.M{
		ResourceTypeKey: resourceType,
		ResourceIdKey:   resourceId,
	})
}

func Find(query db.Q) ([]Comment, error) {
	comments := []Comment{}
	err := db.FindAllQ(Collection, query, &comments)
	if adb.ResultsNotFound(err) {
		return nil, nil
	}
	return comments, err
}
