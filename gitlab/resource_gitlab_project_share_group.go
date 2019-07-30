package gitlab

import (
	"log"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/xanzy/go-gitlab"
)

func resourceGitlabProjectShareGroup() *schema.Resource {
	acceptedAccessLevels := make([]string, 0, len(accessLevelID))
	for k := range accessLevelID {
		if k != "owner" {
			acceptedAccessLevels = append(acceptedAccessLevels, k)
		}
	}
	return &schema.Resource{
		Create: resourceGitlabProjectShareGroupCreate,
		Read:   resourceGitlabProjectShareGroupRead,
		Update: resourceGitlabProjectShareGroupUpdate,
		Delete: resourceGitlabProjectShareGroupDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"project_id": {
				Type:     schema.TypeString,
				ForceNew: true,
				Required: true,
			},
			"group_id": {
				Type:     schema.TypeInt,
				ForceNew: true,
				Required: true,
			},
			"access_level": {
				Type:         schema.TypeString,
				ValidateFunc: validateValueFunc(acceptedAccessLevels),
				Required:     true,
			},
		},
	}
}

func resourceGitlabProjectShareGroupCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*gitlab.Client)

	groupId := d.Get("group_id").(int)
	projectId := d.Get("project_id").(string)
	accessLevelId := accessLevelID[d.Get("access_level").(string)]

	options := &gitlab.ShareWithGroupOptions{
		GroupID:     &groupId,
		GroupAccess: &accessLevelId,
	}
	log.Printf("[DEBUG] create gitlab project membership for %d in %s", options.GroupID, projectId)

	_, err := client.Projects.ShareProjectWithGroup(projectId, options)
	if err != nil {
		return err
	}
	groupIdString := strconv.Itoa(groupId)
	d.SetId(buildTwoPartID(&projectId, &groupIdString))
	return resourceGitlabProjectShareGroupRead(d, meta)
}

func resourceGitlabProjectShareGroupRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*gitlab.Client)
	id := d.Id()
	log.Printf("[DEBUG] read gitlab project projectMember %s", id)

	projectId, groupId, e := projectIdAndGroupIdFromId(id)
	if e != nil {
		return e
	}

	projectInformation, _, err := client.Projects.GetProject(projectId, nil)
	if err != nil {
		return err
	}

	for _, v := range projectInformation.SharedWithGroups {
		if groupId == v.GroupID {
			resourceGitlabProjectShareGroupSetToState(d, v, &projectId)
		}
	}

	return nil
}

func projectIdAndGroupIdFromId(id string) (string, int, error) {
	projectId, groupIdString, err := parseTwoPartID(id)
	groupId, e := strconv.Atoi(groupIdString)
	if err != nil {
		e = err
	}
	if e != nil {
		log.Printf("[WARN] cannot get project member id from input: %v", id)
	}
	return projectId, groupId, e
}

func resourceGitlabProjectShareGroupUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*gitlab.Client)

	groupId := d.Get("group_id").(int)
	projectId := d.Get("project_id").(string)
	accessLevelId := accessLevelID[strings.ToLower(d.Get("access_level").(string))]

	options := gitlab.ShareWithGroupOptions{
		GroupID:     &groupId,
		GroupAccess: &accessLevelId,
	}
	log.Printf("[DEBUG] update gitlab project membership %v for %s", groupId, projectId)

	_, err := client.Projects.ShareProjectWithGroup(projectId, &options)
	if err != nil {
		return err
	}
	return resourceGitlabProjectShareGroupRead(d, meta)
}

func resourceGitlabProjectShareGroupDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*gitlab.Client)

	id := d.Id()
	projectId, groupId, e := projectIdAndGroupIdFromId(id)
	if e != nil {
		return e
	}

	log.Printf("[DEBUG] Delete gitlab project membership %v for %s", groupId, projectId)

	_, err := client.Projects.DeleteSharedProjectFromGroup(projectId, groupId)
	return err
}

func resourceGitlabProjectShareGroupSetToState(d *schema.ResourceData, group struct {
	GroupID          int    "json:\"group_id\""
	GroupName        string "json:\"group_name\""
	GroupAccessLevel int    "json:\"group_access_level\""
}, projectId *string) {

	//This cast is needed due to an inconsistency in the upstream API
	//GroupAcessLevel is returned as an int but the map we lookup is sorted by the int alias AccessLevelValue
	convertedAccessLevel := gitlab.AccessLevelValue(group.GroupAccessLevel)

	d.Set("project_id", projectId)
	d.Set("group_id", group.GroupID)
	d.Set("access_level", accessLevel[convertedAccessLevel])

	groupId := strconv.Itoa(group.GroupID)
	d.SetId(buildTwoPartID(projectId, &groupId))
}
