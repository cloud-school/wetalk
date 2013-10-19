// Copyright 2013 wetalk authors
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package routers

import (
	"fmt"

	"github.com/astaxie/beego/orm"

	"github.com/beego/wetalk/models"
)

// HomeRouter serves home page.
type PostRouter struct {
	baseRouter
}

func (this *PostRouter) setCategories(cats *[]models.Category) {
	models.ListCategories(cats)
	this.Data["Categories"] = *cats
}

func (this *PostRouter) setTopicsOfCat(topics *[]models.Topic, cat *models.Category) {
	models.ListTopicsOfCat(topics, cat)
	this.Data["Topics"] = *topics
}

// Get implemented Get method for HomeRouter.
func (this *PostRouter) Home() {
	this.Data["IsHome"] = true
	this.TplNames = "post/home.html"

	var cats []models.Category
	this.setCategories(&cats)

	var posts []models.Post
	qs := models.Posts().OrderBy("-Created").Limit(50).RelatedSel()
	models.ListObjects(qs, &posts)
	this.Data["Posts"] = posts

	this.Data["CategorySlug"] = "hot"
}

// Get implemented Get method for HomeRouter.
func (this *PostRouter) Category() {
	this.TplNames = "post/category.html"

	slug := this.GetString(":slug")
	cat := models.Category{Slug: slug}
	if err := cat.Read("Slug"); err != nil {
		this.Abort("404")
		return
	}

	pers := 25

	qs := models.Posts().Filter("Category", &cat)

	cnt, _ := models.CountObjects(qs)
	pager := this.SetPaginator(pers, cnt)

	qs = qs.OrderBy("-Created").Limit(pers, pager.Offset()).RelatedSel()

	var posts []models.Post
	models.ListObjects(qs, &posts)

	this.Data["Posts"] = posts
	this.Data["Category"] = &cat
	this.Data["CategorySlug"] = cat.Slug

	var cats []models.Category
	this.setCategories(&cats)

	var topics []models.Topic
	this.setTopicsOfCat(&topics, &cat)
}

// Get implemented Get method for HomeRouter.
func (this *PostRouter) Navs() {
	slug := this.GetString(":slug")

	switch slug {
	case "favs", "follow":
		if this.CheckLoginRedirect() {
			return
		}
	}

	this.Data["CategorySlug"] = slug
	this.TplNames = fmt.Sprintf("post/navs/%s.html", slug)

	pers := 25

	var posts []models.Post

	switch slug {
	case "recent":
		qs := models.Posts()

		cnt, _ := models.CountObjects(qs)
		pager := this.SetPaginator(pers, cnt)

		qs = qs.OrderBy("-Updated").Limit(pers, pager.Offset()).RelatedSel()

		models.ListObjects(qs, &posts)

		var cats []models.Category
		this.setCategories(&cats)

	case "best":
		qs := models.Posts().Filter("IsBest", true)

		cnt, _ := models.CountObjects(qs)
		pager := this.SetPaginator(pers, cnt)

		qs = qs.OrderBy("-Created").Limit(pers, pager.Offset()).RelatedSel()

		models.ListObjects(qs, &posts)

		var cats []models.Category
		this.setCategories(&cats)

	case "cold":
		qs := models.Posts().Filter("Replys", 0)

		cnt, _ := models.CountObjects(qs)
		pager := this.SetPaginator(pers, cnt)

		qs = qs.OrderBy("-Created").Limit(pers, pager.Offset()).RelatedSel()

		models.ListObjects(qs, &posts)

		var cats []models.Category
		this.setCategories(&cats)

	case "favs":
		var topicIds orm.ParamsList
		nums, _ := models.FollowTopics().Filter("User", &this.user.Id).OrderBy("-Created").ValuesFlat(&topicIds, "Topic")
		if nums > 0 {
			qs := models.Posts().Filter("Topic__in", topicIds)

			cnt, _ := models.CountObjects(qs)
			pager := this.SetPaginator(pers, cnt)

			qs = qs.OrderBy("-Created").Limit(pers, pager.Offset()).RelatedSel()

			models.ListObjects(qs, &posts)

			var topics []models.Topic
			models.Topics().Filter("Id__in", topicIds).Limit(8).All(&topics)
			this.Data["Topics"] = topics
		}

	case "follow":
		var userIds orm.ParamsList
		nums, _ := models.Follows().Filter("User", &this.user.Id).OrderBy("-Created").ValuesFlat(&userIds, "FollowUser")
		if nums > 0 {
			qs := models.Posts().Filter("User__in", userIds)

			cnt, _ := models.CountObjects(qs)
			pager := this.SetPaginator(pers, cnt)

			qs = qs.OrderBy("-Created").Limit(pers, pager.Offset()).RelatedSel()

			models.ListObjects(qs, &posts)

			var followUsers []models.User
			models.Users().Filter("Id__in", userIds).Limit(8).All(&followUsers)
			this.Data["FollowUsers"] = followUsers
		}
	}

	this.Data["Posts"] = posts
}

// Get implemented Get method for HomeRouter.
func (this *PostRouter) Topic() {
	slug := this.GetString(":slug")

	switch slug {
	default: // View topic.
		this.TplNames = "post/topic.html"
		topic := models.Topic{Slug: slug}
		if err := topic.Read("Slug"); err != nil {
			this.Abort("404")
			return
		}

		pers := 25

		qs := models.Posts().Filter("Topic", &topic)

		cnt, _ := models.CountObjects(qs)
		pager := this.SetPaginator(pers, cnt)

		qs = qs.OrderBy("-Created").Limit(pers, pager.Offset()).RelatedSel()

		var posts []models.Post
		models.ListObjects(qs, &posts)

		this.Data["Posts"] = posts
		this.Data["Topic"] = &topic
		this.Data["IsTopic"] = true

		HasFavorite := false
		if this.isLogin {
			HasFavorite = models.FollowTopics().Filter("User", &this.user).Filter("Topic", &topic).Exist()
		}
		this.Data["HasFavorite"] = HasFavorite
	}
}

// Get implemented Get method for HomeRouter.
func (this *PostRouter) TopicSubmit() {
	slug := this.GetString(":slug")

	topic := models.Topic{Slug: slug}
	if err := topic.Read("Slug"); err != nil {
		this.Abort("404")
		return
	}

	result := map[string]interface{}{
		"success": false,
	}

	if this.IsAjax() {
		action := this.GetString("action")
		switch action {
		case "favorite":
			if !this.FormOnceNotMatch() && this.isLogin {
				qs := models.FollowTopics().Filter("User", &this.user).Filter("Topic", &topic)
				if qs.Exist() {
					qs.Delete()
				} else {
					fav := models.FollowTopic{User: &this.user, Topic: &topic}
					fav.Insert()
				}
				topic.RefreshFollowers()
				this.user.RefreshFavTopics()
				result["once"] = this.Data["once_token"]
				result["success"] = true
			}
		}
	}

	this.Data["json"] = result
	this.ServeJson()
}

func (this *PostRouter) New() {
	this.TplNames = "post/new.html"

	if this.CheckActiveRedirect() {
		return
	}

	form := models.PostForm{}

	slug := this.GetString("topic")
	if len(slug) > 0 {
		topic := models.Topic{Slug: slug}
		topic.Read("Slug")
		form.Topic = topic.Id
		this.Data["Topic"] = &topic
	}

	models.ListCategories(&form.Categories)
	models.ListTopics(&form.Topics)
	this.SetFormSets(&form)
}

func (this *PostRouter) NewSubmit() {
	this.TplNames = "post/new.html"

	if this.CheckActiveRedirect() {
		return
	}

	if this.IsAjax() {
		result := map[string]interface{}{
			"success": false,
		}
		if !this.FormOnceNotMatch() {
			action := this.GetString("action")
			switch action {
			case "preview":
				content := this.GetString("content")
				result["preview"] = models.RenderPostContent(content)
				result["once"] = this.Data["once_token"]
				result["success"] = true
			}
		}
		this.Data["json"] = result
		this.ServeJson()
		return
	}

	form := models.PostForm{}
	slug := this.GetString("topic")
	if len(slug) > 0 {
		topic := models.Topic{Slug: slug}
		topic.Read("Slug")
		form.Topic = topic.Id
		this.Data["Topic"] = &topic
	}

	models.ListCategories(&form.Categories)
	models.ListTopics(&form.Topics)
	if !this.ValidFormSets(&form) {
		return
	}

	var post models.Post
	if err := form.SavePost(&post, &this.user); err == nil {
		this.Redirect(post.Link(), 302)
	} else {
		fmt.Println(err)
	}
}

func (this *PostRouter) Single() {
	this.TplNames = "post/post.html"

	var post models.Post
	id, _ := this.GetInt(":post")
	if id > 0 {
		models.Posts().Filter("Id", id).RelatedSel().One(&post)
	}

	if post.Id == 0 {
		this.Abort("404")
		return
	}

	this.Data["Post"] = &post
}

func (this *PostRouter) SingleSubmit() {
	if this.CheckActiveRedirect() {
		return
	}

	var post models.Post
	id, _ := this.GetInt(":post")
	if id > 0 {
		post.Id = int(id)
		post.Read()
	}

	if post.Id == 0 {
		this.Abort("404")
		return
	}
}