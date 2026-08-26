package goods

import (
	"context"
	"encoding/json"
	"github.com/samber/lo"
	"go.uber.org/zap"
	"mall/adaptor/repo/model"
	"mall/common"
	"mall/consts"
	"mall/service/do"
	"mall/service/dto"
	"mall/utils/logger"
	"mall/utils/pool"
	"sort"
)

func (s *Service) CreateCourse(ctx context.Context, user *common.AdminUser, req *dto.CreateCourseReq) (int64, common.Errno) {
	courseID, err := s.course.CreateCourse(ctx, &do.CreateCourse{
		UserID:         user.UserID,
		Name:           req.Name,
		CoursePrice:    req.CoursePrice,
		ServiceTime:    req.ServiceTime,
		LearnTime:      req.LearnTime,
		Sort:           req.Sort,
		Features:       req.Features,
		UpdateStatus:   req.UpdateStatus,
		CoverKey:       req.CoverKey,
		DetailCoverKey: req.DetailCoverKey,
		Detail:         req.Detail,
	})
	if err != nil {
		logger.Error("CreateCourse CreateCourse error", zap.Any("req", req), zap.Error(err))
		return 0, common.DatabaseErr.WithErr(err)
	}
	if derr := s.rdsCourse.DelCourseInfo(ctx, courseID); derr != nil {
		logger.Error("CreateCourse DelCourseInfo error", zap.Error(derr), zap.Int64("id", courseID))
	}
	return courseID, common.OK
}

func (s *Service) GetCourseInfo(ctx context.Context, req *dto.CourseInfoReq) (*dto.CourseDto, common.Errno) {
	course, err := s.getCourseInfoCached(ctx, req.ID)
	if err != nil {
		logger.Error("GetCourseInfo GetCourseInfoById error", zap.Any("req", req), zap.Error(err))
		return nil, common.DatabaseErr.WithErr(err)
	}

	var (
		fileKeys   = []string{course.CoverKey, course.DetailCoverKey}
		fileUrlMap = make(map[string]string)
	)
	fileUrlMap, err = s.storage.GetPreviewUrl(ctx, &do.GetPreviewUrl{
		Keys:        fileKeys,
		ExpireHours: 6,
	})
	if err != nil {
		logger.Error("GetCourseInfo GetPreviewUrl error", zap.Error(err), zap.Any("fileKeys", fileKeys))
		return nil, common.ServerErr.WithErr(err)
	}
	var features []string
	json.Unmarshal([]byte(course.Features), &features)
	return &dto.CourseDto{
		ID:             course.ID,
		Name:           course.Name,
		CoursePrice:    course.CoursePrice,
		ServiceTime:    course.ServiceTime,
		LearnTime:      course.LearnTime,
		Sort:           course.Sort,
		Status:         course.Status,
		Features:       features,
		UpdateStatus:   course.UpdateStatus,
		CoverKey:       course.CoverKey,
		CoverUrl:       fileUrlMap[course.CoverKey],
		Detail:         course.Detail,
		DetailCoverKey: course.DetailCoverKey,
		DetailCoverUrl: fileUrlMap[course.DetailCoverKey],
	}, common.OK
}

func (s *Service) UpdateCourse(ctx context.Context, user *common.AdminUser, req *dto.UpdateCourseReq) common.Errno {
	err := s.course.UpdateCourse(ctx, &do.UpdateCourse{
		UserID:         user.UserID,
		ID:             req.ID,
		Name:           req.Name,
		CoursePrice:    req.CoursePrice,
		ServiceTime:    req.ServiceTime,
		LearnTime:      req.LearnTime,
		Sort:           req.Sort,
		Features:       req.Features,
		UpdateStatus:   req.UpdateStatus,
		CoverKey:       req.CoverKey,
		DetailCoverKey: req.DetailCoverKey,
		Detail:         req.Detail,
	})
	if err != nil {
		logger.Error("UpdateCourse UpdateCourse error", zap.Any("req", req), zap.Error(err))
		return common.DatabaseErr.WithErr(err)
	}
	if derr := s.rdsCourse.DelCourseInfo(ctx, req.ID); derr != nil {
		logger.Error("UpdateCourse DelCourseInfo error", zap.Error(derr), zap.Int64("id", req.ID))
	}
	return common.OK
}

func (s *Service) UpdateCourseStatus(ctx context.Context, user *common.AdminUser, req *dto.UpdateCourseStatusReq) common.Errno {
	err := s.course.UpdateCourseStatus(ctx, &do.UpdateCourseStatus{
		ID:     req.ID,
		Status: req.Status,
		UserID: user.UserID,
	})
	if err != nil {
		logger.Error("UpdateCourseStatus UpdateCourseStatus error", zap.Any("req", req), zap.Error(err))
		return common.DatabaseErr.WithErr(err)
	}
	if derr := s.rdsCourse.DelCourseInfo(ctx, req.ID); derr != nil {
		logger.Error("UpdateCourseStatus DelCourseInfo error", zap.Error(derr), zap.Int64("id", req.ID))
	}
	return common.OK
}

func (s *Service) ListCourse(ctx context.Context, req *dto.CourseListReq) (*dto.CourseListResp, common.Errno) {
	list, count, err := s.course.ListCourse(ctx, &do.CourseList{
		Pager:           req.Pager,
		ID:              req.ID,
		UserID:          req.UserID,
		NameKw:          req.NameKw,
		StartCreateTime: req.CreateStartTime,
		EndCreateTime:   req.CreateEndTime,
		StartUpdateTime: req.UpdateStartTime,
		EndUpdateTime:   req.UpdateEndTime,
		UpdateStatus:    req.UpdateStatus,
		Status:          req.Status,
	})
	if err != nil {
		logger.Error("ListCourse ListCourse error", zap.Any("req", req), zap.Error(err))
		return nil, common.DatabaseErr.WithErr(err)
	}

	retList := s.convertCoursesToDto(ctx, list)
	return &dto.CourseListResp{
		List:  retList,
		Total: count,
		Pager: req.Pager,
	}, common.OK
}

func (s *Service) convertCoursesToDto(ctx context.Context, list []*model.CourseGood) []*dto.CourseDto {
	var (
		fileKeys    = make([]string, 0)
		fileUrlMap  = make(map[string]string)
		userNameMap = make(map[int64]string)
		userIds     = make([]int64, 0)
	)

	lo.ForEach(list, func(v *model.CourseGood, index int) {
		fileKeys = append(fileKeys, v.CoverKey, v.DetailCoverKey)
		userIds = append(userIds, v.CreateBy, v.UpdateBy)
	})

	tempPool := pool.NewPoolWithSize(2)
	defer tempPool.Release()
	tempPool.RunGo(func() {
		tempMap, err := s.storage.GetPreviewUrl(ctx, &do.GetPreviewUrl{
			Keys:        fileKeys,
			ExpireHours: 6,
		})
		if err != nil {
			logger.Error("convertCoursesToDto GetPreviewUrl error", zap.Error(err), zap.Any("fileKeys", fileKeys))
			return
		}
		fileUrlMap = tempMap
	})
	tempPool.RunGo(func() {
		temp, err := s.adminUser.GetUserNameMap(ctx, userIds)
		if err != nil {
			logger.Error("convertCoursesToDto GetUserNameMap error", zap.Error(err), zap.Any("userIds", userIds))
			return
		}
		userNameMap = temp
	})
	tempPool.Wait()
	retList := make([]*dto.CourseDto, 0)
	for _, v := range list {
		features := []string{}
		json.Unmarshal([]byte(v.Features), &features)
		retList = append(retList, &dto.CourseDto{
			ID:             v.ID,
			Name:           v.Name,
			CoursePrice:    v.CoursePrice,
			ServiceTime:    v.ServiceTime,
			LearnTime:      v.LearnTime,
			Sort:           v.Sort,
			Status:         v.Status,
			Features:       features,
			UpdateStatus:   v.UpdateStatus,
			CoverKey:       v.CoverKey,
			CoverUrl:       fileUrlMap[v.CoverKey],
			Detail:         v.Detail,
			DetailCoverKey: v.DetailCoverKey,
			DetailCoverUrl: fileUrlMap[v.DetailCoverKey],
			CreateAt:       v.CreateAt.UnixMilli(),
			UpdateAt:       v.UpdateAt.UnixMilli(),
			CreateBy:       v.CreateBy,
			UpdateBy:       v.UpdateBy,
			CreateUpdateName: common.CreateUpdateName{
				CreateName: userNameMap[v.CreateBy],
				UpdateName: userNameMap[v.UpdateBy],
			},
		})
	}
	return retList
}

func (s *Service) GetCourseDetail(ctx context.Context, req *dto.CourseInfoReq) (*dto.CourseDetailDto, common.Errno) {
	courseDto, errno := s.GetCourseInfo(ctx, req)
	if errno.NotOk() {
		logger.Error("GetCourseDetail GetCourseInfo error", zap.Any("req", req), zap.Error(errno))
		return nil, errno
	}
	courseCatalogDto, errno := s.CatalogInfo(ctx, &dto.CatalogInfoReq{
		CourseID:    req.ID,
		FilterVideo: true,
	})
	if errno.NotOk() {
		logger.Error("GetCourseDetail CatalogInfo error", zap.Any("req", req), zap.Error(errno))
		return nil, errno
	}
	return &dto.CourseDetailDto{
		CourseDto:       courseDto,
		CatalogInfoResp: courseCatalogDto,
	}, common.OK
}

func (s *Service) AddCatalog(ctx context.Context, user *common.AdminUser, req *dto.AddCatalogReq) (int64, common.Errno) {
	catalogID, err := s.course.AddCatalog(ctx, &do.AddCatalog{
		CourseID: req.CourseID,
		Level:    req.Level,
		Name:     req.Name,
		ParentID: req.ParentID,
		Sort:     req.Sort,
		UserID:   user.UserID,
	})
	if err != nil {
		logger.Error("AddCatalog AddCatalog error", zap.Any("req", req), zap.Error(err))
		return 0, common.DatabaseErr.WithErr(err)
	}
	return catalogID, common.OK
}

func (s *Service) UpdateCatalog(ctx context.Context, user *common.AdminUser, req *dto.UpdateCatalogReq) common.Errno {
	err := s.course.UpdateCatalog(ctx, &do.UpdateCatalog{
		ID:     req.ID,
		Name:   req.Name,
		UserID: user.UserID,
	})
	if err != nil {
		logger.Error("UpdateCatalog UpdateCatalog error", zap.Any("req", req), zap.Error(err))
		return common.DatabaseErr.WithErr(err)
	}
	return common.OK
}

func (s *Service) DeleteCatalog(ctx context.Context, user *common.AdminUser, req *dto.DeleteCatalogReq) common.Errno {
	err := s.course.DeleteCatalog(ctx, &do.DeleteCatalog{
		ID: req.ID,
	})
	if err != nil {
		logger.Error("DeleteCatalog DeleteCatalog error", zap.Any("req", req), zap.Error(err))
		return common.DatabaseErr.WithErr(err)
	}
	return common.OK
}

func (s *Service) UpdateCatalogSort(ctx context.Context, user *common.AdminUser, sortList []dto.UpdateCatalogSortDto) common.Errno {
	doSortList := make([]*do.CatalogSort, 0)
	lo.ForEach(sortList, func(item dto.UpdateCatalogSortDto, index int) {
		doSortList = append(doSortList, &do.CatalogSort{
			Id:       item.Id,
			Lessons:  item.Lessons,
			Level:    item.Level,
			ParentId: item.ParentId,
			Sort:     item.Sort,
		})
	})
	err := s.course.UpdateCatalogSort(ctx, &do.UpdateCatalogSort{
		SortList: doSortList,
		UserID:   user.UserID,
	})
	if err != nil {
		logger.Error("UpdateCatalogSort UpdateCatalogSort error", zap.Any("sortList", sortList), zap.Error(err))
		return common.DatabaseErr.WithErr(err)
	}
	return common.OK
}

func (s *Service) CatalogInfo(ctx context.Context, req *dto.CatalogInfoReq) (*dto.CatalogInfoResp, common.Errno) {
	if req.CourseID <= 0 {
		return &dto.CatalogInfoResp{
			TotalDuration: 0,
			LessonCount:   0,
			Catalogs:      nil,
		}, common.OK
	}
	catalogList, err := s.course.GetCatalogsByCourseId(ctx, req.CourseID)
	if err != nil {
		logger.Error("CatalogInfo GetCatalogsByCourseId error", zap.Any("req", req), zap.Error(err))
		return nil, common.DatabaseErr.WithErr(err)
	}
	courseLessons, err := s.course.GetCourseLessons(ctx, req.CourseID)
	if err != nil {
		logger.Error("CatalogInfo GetCourseLessons error", zap.Any("req", req), zap.Error(err))
		return nil, common.DatabaseErr.WithErr(err)
	}
	lessonIds := make([]int64, 0)
	catalogLessonGroup := lo.GroupBy(courseLessons, func(v *model.CourseLesson) int64 {
		lessonIds = append(lessonIds, v.LessonID)
		return v.CatalogID
	})

	lessonMap, err := s.lesson.GetLessonByIds(ctx, lessonIds)
	if err != nil {
		logger.Error("CatalogInfo GetLessonByIds error", zap.Any("req", req), zap.Error(err))
		return nil, common.DatabaseErr.WithErr(err)
	}

	fileUrlMap := make(map[string]string)
	if !req.FilterVideo {
		videoKeys := lo.Map(courseLessons, func(v *model.CourseLesson, index int) string {
			return lessonMap[v.LessonID].VideoKey
		})

		fileUrlMap, err = s.storage.GetPreviewUrl(ctx, &do.GetPreviewUrl{
			Keys:        videoKeys,
			ExpireHours: 12,
		})
		if err != nil {
			logger.Error("CatalogInfo GetFileUrls error", zap.Any("req", req), zap.Error(err))
			return nil, common.DatabaseErr.WithErr(err)
		}
	}
	var (
		retCatalogList = make([]*dto.CatalogDto, 0)
		lessonIndex    = 0
		totalDuration  int64
	)

	for _, catalog := range catalogList {
		lessonDtos := make([]*dto.CatalogLessonDto, 0)
		lessons, _ := catalogLessonGroup[catalog.ID]
		sort.Slice(lessons, func(i, j int) bool {
			return lessons[i].Sort < lessons[j].Sort
		})
		lessonDtos = append(lessonDtos, lo.Map(lessons, func(v *model.CourseLesson, index int) *dto.CatalogLessonDto {
			lessonIndex++
			totalDuration = totalDuration + int64(lessonMap[v.LessonID].Duration)
			return &dto.CatalogLessonDto{
				ID:            v.ID,
				Index:         lessonIndex,
				LessonID:      v.LessonID,
				LessonName:    lessonMap[v.LessonID].Name,
				Name:          v.Name,
				Detail:        lessonMap[v.LessonID].Detail,
				VideoUrl:      fileUrlMap[lessonMap[v.LessonID].VideoKey],
				VideoFileName: lessonMap[v.LessonID].VideoFileName,
				Duration:      lessonMap[v.LessonID].Duration,
				Status:        lessonMap[v.LessonID].Status,
				ShowTime:      v.ShowTime.UnixMilli(),
				EnableTrial:   v.EnableTrial,
			}
		})...)
		retCatalogList = append(retCatalogList, &dto.CatalogDto{
			CourseID:    catalog.CourseID,
			ID:          catalog.ID,
			Name:        catalog.Name,
			Level:       catalog.Level,
			ParentID:    catalog.ParentID,
			Sort:        catalog.Sort,
			Lessons:     lessonDtos,
			LessonCount: int64(len(lessonDtos)),
		})
	}
	return &dto.CatalogInfoResp{
		TotalDuration: totalDuration,
		LessonCount:   int64(lessonIndex),
		Catalogs:      retCatalogList,
	}, common.OK
}

func (s *Service) AddCatalogLesson(ctx context.Context, user *common.AdminUser, req *dto.AddCatalogLessonReq) common.Errno {
	lessonMap, err := s.lesson.GetLessonByIds(ctx, req.LessonIDs)
	if err != nil {
		logger.Error("AddCatalogLesson GetLessonByIds error", zap.Any("req", req), zap.Error(err))
		return common.DatabaseErr.WithErr(err)
	}
	lessonNameMap := make(map[int64]string)
	for lessonID, lesson := range lessonMap {
		lessonNameMap[lessonID] = lesson.Name
	}
	err = s.course.AddCatalogLesson(ctx, &do.AddCatalogLesson{
		CatalogID: req.CatalogID,
		CourseID:  req.CourseID,
		LessonMap: lessonNameMap,
		UserID:    user.UserID,
	})
	if err != nil {
		logger.Error("AddCatalogLesson AddCatalogLesson error", zap.Any("req", req), zap.Error(err))
		return common.DatabaseErr.WithErr(err)
	}
	return common.OK
}

func (s *Service) UpdateCatalogLesson(ctx context.Context, user *common.AdminUser, req *dto.UpdateCatalogLessonReq) common.Errno {
	err := s.course.UpdateCatalogLesson(ctx, &do.UpdateCatalogLesson{
		ID:          req.ID,
		EnableTrial: req.EnableTrial,
		Name:        req.Name,
		UserID:      user.UserID,
	})
	if err != nil {
		logger.Error("UpdateCatalogLesson UpdateCatalogLesson error", zap.Any("req", req), zap.Error(err))
		return common.DatabaseErr.WithErr(err)
	}
	return common.OK
}

func (s *Service) RemoveCatalogLesson(ctx context.Context, user *common.AdminUser, req *dto.RemoveCatalogLessonReq) common.Errno {
	err := s.course.RemoveCatalogLesson(ctx, &do.RemoveCatalogLesson{IDs: req.IDs})
	if err != nil {
		logger.Error("RemoveCatalogLesson RemoveCatalogLesson error", zap.Any("req", req), zap.Error(err))
		return common.DatabaseErr.WithErr(err)
	}
	return common.OK
}

func (s *Service) GetPurchasedCourseList(ctx context.Context, user *common.UserInfo, req *dto.GetPurchasedCourseReq) (*dto.GetPurchasedCourseResp, common.Errno) {
	list, count, err := s.userCourse.ListUserCourse(ctx, &do.ListUserCourse{
		UserId: user.User.ID,
		Pager:  req.Pager,
	})
	if err != nil {
		logger.Error("GetPurchasedCourseList ListUserCourse error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	goodsIds := make([]int64, 0)
	lo.ForEach(list, func(item *model.UserCourseGood, index int) {
		goodsIds = append(goodsIds, item.GoodsID)
	})
	goodsList, err := s.course.GetCourseInfoByIds(ctx, goodsIds)
	if err != nil {
		logger.Error("GetPurchasedCourseList GetCourseInfoByIds error", zap.Error(err), zap.Any("req", req))
		return nil, common.DatabaseErr.WithErr(err)
	}
	fileKeys := make([]string, 0)
	retList := make([]*dto.PurchasedCourseDto, 0)
	goodsMap := lo.SliceToMap(goodsList, func(item *model.CourseGood) (int64, *model.CourseGood) {
		fileKeys = append(fileKeys, item.CoverKey, item.DetailCoverKey)
		return item.ID, item
	})
	fileUrlMap, err := s.storage.GetPreviewUrl(ctx, &do.GetPreviewUrl{Keys: fileKeys})
	if err != nil {
		logger.Error("GetPurchasedCourseList GetPreviewUrl error", zap.Error(err), zap.Any("req", req))
		return nil, common.ServerErr.WithErr(err)
	}
	for _, item := range list {
		goods, ok := goodsMap[item.GoodsID]
		if !ok {
			continue
		}
		features := make([]string, 0)
		json.Unmarshal([]byte(goods.Features), &features)

		retList = append(retList, &dto.PurchasedCourseDto{
			ID:                item.GoodsID,
			Name:              goods.Name,
			ServiceExpireTime: item.ServiceExpireTime,
			LearnExpireTime:   item.LearnExpireTime,
			Features:          features,
			UpdateStatus:      goods.UpdateStatus,
			CoverKey:          goods.CoverKey,
			CoverUrl:          fileUrlMap[goods.CoverKey],
			DetailCoverKey:    goods.DetailCoverKey,
			DetailCoverUrl:    fileUrlMap[goods.DetailCoverKey],
			Detail:            goods.Detail,
		})
	}
	return &dto.GetPurchasedCourseResp{
		List:  retList,
		Total: count,
		Pager: req.Pager,
	}, common.OK
}

func (s *Service) getCourseInfoCached(ctx context.Context, id int64) (*model.CourseGood, error) {
	cacheStr, err := s.rdsCourse.GetCourseInfo(ctx, id)
	if err == nil {
		//命中，cacheStr里是一段JSON
		course := &model.CourseGood{}
		if jerr := json.Unmarshal([]byte(cacheStr), course); jerr == nil {
			return course, nil
		}
	}
	course, err := s.course.GetCourseInfoById(ctx, id)
	if err != nil {
		return nil, err
	}
	data, _ := json.Marshal(course)
	if serr := s.rdsCourse.SetCourseInfo(ctx, id, string(data), consts.CourseInfoCacheExpire); serr != nil {
		logger.FromContext(ctx).Error("getCourseInfoCached SetCourseInfo error", zap.Error(serr), zap.Int64("id", id))
	}
	return course, nil

}
