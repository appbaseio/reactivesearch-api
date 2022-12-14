package reindex

import (
	"context"
	"encoding/json"
	"time"

	"github.com/appbaseio/reactivesearch-api/util"
	es7 "github.com/olivere/elastic/v7"
	log "github.com/sirupsen/logrus"
	es6 "gopkg.in/olivere/elastic.v6"
)

type SetAliasConfig struct {
	AliasName    string
	NewIndex     string
	OldIndex     string
	IsWriteIndex bool
}

const logTag = "[reindex]"

const typeName = "_doc"

func setAliasEs7(tenantId string, aliasConfig SetAliasConfig) error {
	aliasConfig.OldIndex = util.AppendTenantID(aliasConfig.OldIndex, tenantId)
	// Get the client ready for the request
	//
	// If the request is for a multi-tenant setup and the backend
	// is `system`, we need to use the system client to make the call.
	esClient, clientFetchErr := util.GetESClientForTenant(context.Background())
	if clientFetchErr != nil {
		log.Warnln(logTag, ": ", clientFetchErr)
		return clientFetchErr
	}
	// Delete source index, we need to first delete the index
	// because there can be only one write index at a particular time
	_, err := esClient.DeleteIndex(aliasConfig.OldIndex).
		Do(context.Background())

	if err != nil {
		log.Errorln(logTag, ":", err)
		return err
	}
	// Add the alias to destination index
	_, err2 := esClient.Alias().Action(
		es7.NewAliasAddAction(aliasConfig.AliasName).
			Index(aliasConfig.NewIndex).
			IsWriteIndex(aliasConfig.IsWriteIndex),
	).Do(context.Background())
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return err2
	}
	return nil
}

func setAliasEs6(aliasConfig SetAliasConfig) error {
	// Delete source index
	_, err := util.GetClient6().Delete().Index(aliasConfig.OldIndex).
		Do(context.Background())
	if err != nil {
		log.Errorln(logTag, ":", err)
		return err
	}
	// Add the alias to destination index
	_, err2 := util.GetClient6().Alias().Action(
		es6.NewAliasAddAction(aliasConfig.AliasName).
			Index(aliasConfig.NewIndex).
			IsWriteIndex(aliasConfig.IsWriteIndex),
	).Do(context.Background())
	if err2 != nil {
		log.Errorln(logTag, ":", err2)
		return err2
	}
	return nil
}

// Set alias to an index
func SetAlias(tenantId string, aliasConfig SetAliasConfig) error {
	return setAliasEs7(tenantId, aliasConfig)
}

// To track a re-index task by taskID
func IsTaskCompleted(ctx context.Context, taskID string) (bool, error) {
	res := false
	// Get the client ready for the request
	//
	// If the request is for a multi-tenant setup and the backend
	// is `system`, we need to use the system client to make the call.
	esClient, clientFetchErr := util.GetESClientForTenant(ctx)
	if clientFetchErr != nil {
		log.Warnln(logTag, ": ", clientFetchErr)
		return res, clientFetchErr
	}
	status, err := esClient.TasksGetTask().TaskId(taskID).Do(ctx)
	if err != nil {
		log.Errorln(logTag, " Get task status error", err)
		return res, err
	}

	res = status.Completed
	return res, nil
}

// To track reindex process. Use it in a go routine to track asynchronously.
func TrackReindex(tenantId string, aliasConfig SetAliasConfig, taskDetails []byte) {
	isCompleted := make(chan bool, 1)
	ticker := time.Tick(30 * time.Second)
	ctx := context.Background()
	var taskInfo *es7.StartTaskResult
	err := json.Unmarshal(taskDetails, &taskInfo)
	if err != nil {
		log.Errorln(logTag, "Error encountered while un-marshalling re-index task response", err)
	}
	taskId := taskInfo.TaskId
	for {
		select {
		case <-ticker:
			ok, _ := IsTaskCompleted(ctx, taskId)
			log.Println(logTag, " "+taskId+" task is still re-indexing data...")
			if ok {
				isCompleted <- true
			}
		case <-isCompleted:
			log.Println(logTag, taskId+" task completed successfully")
			err := SetAlias(tenantId, aliasConfig)
			if err != nil {
				log.Errorln(logTag, " post re-indexing error: ", err)
			}
			return
		}
	}
}
