package goods

import (
	"context"
	"errors"
	"github.com/go-redis/redis"
	"mall/adaptor/repo/model"
	"mall/service/do"
	"testing"
	"time"
)

type fakeCourse struct {
	dbCourse *model.CourseGood
	dbErr    error
	called   int
}
type fakeRedis struct {
	cache  map[int64]string
	setErr error
}

func (f *fakeRedis) GetCourseInfo(ctx context.Context, id int64) (string, error) {
	v, ok := f.cache[id]
	if !ok {
		return "", redis.Nil
	}
	return v, nil
}
func (f *fakeRedis) SetCourseInfo(ctx context.Context, id int64, info string, expire time.Duration) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.cache[id] = info
	return nil
}
func (f *fakeRedis) DelCourseInfo(ctx context.Context, id int64) error {
	f.cache[id] = ""
	return nil
}
func (f *fakeCourse) CreateCourse(ctx context.Context, req *do.CreateCourse) (int64, error) {
	return 0, nil
}
func (f *fakeCourse) GetCourseInfoById(ctx context.Context, id int64) (*model.CourseGood, error) {
	f.called++
	return f.dbCourse, f.dbErr
}
func (f *fakeCourse) GetCourseInfoByIds(ctx context.Context, ids []int64) ([]*model.CourseGood, error) {
	return nil, nil
}
func (f *fakeCourse) UpdateCourse(ctx context.Context, req *do.UpdateCourse) error {
	return nil
}
func (f *fakeCourse) UpdateCourseStatus(ctx context.Context, req *do.UpdateCourseStatus) error {
	return nil
}
func (f *fakeCourse) ListCourse(ctx context.Context, req *do.CourseList) ([]*model.CourseGood, int64, error) {
	return nil, 0, nil
}

func (f *fakeCourse) AddCatalog(ctx context.Context, req *do.AddCatalog) (int64, error) {
	return 0, nil
}
func (f *fakeCourse) DeleteCatalog(ctx context.Context, req *do.DeleteCatalog) error { return nil }
func (f *fakeCourse) UpdateCatalogSort(ctx context.Context, req *do.UpdateCatalogSort) error {
	return nil
}
func (f *fakeCourse) UpdateCatalog(ctx context.Context, req *do.UpdateCatalog) error { return nil }
func (f *fakeCourse) AddCatalogLesson(ctx context.Context, req *do.AddCatalogLesson) error {
	return nil
}
func (f *fakeCourse) UpdateCatalogLesson(ctx context.Context, req *do.UpdateCatalogLesson) error {
	return nil
}
func (f *fakeCourse) RemoveCatalogLesson(ctx context.Context, req *do.RemoveCatalogLesson) error {
	return nil
}

func (f *fakeCourse) GetCourseLessons(ctx context.Context, courseId int64) ([]*model.CourseLesson, error) {
	return nil, nil
}
func (f *fakeCourse) GetCatalogsByCourseId(ctx context.Context, courseId int64) ([]*model.CourseCatalog, error) {
	return nil, nil
}

func TestGetCourseInfoCached(t *testing.T) {
	ctx := context.Background()
	id := int64(100)

	dbCourse := &model.CourseGood{ID: id, Name: "Go高级实战"}

	cases := []struct {
		name         string
		cacheData    string
		dbCourse     *model.CourseGood
		dbErr        error
		wantName     string
		wantErr      bool
		wantDbCalls  int
		wantCacheSet bool
	}{
		{
			name:         "缓存命中，不查DB",
			cacheData:    `{"ID": 100, "Name":"Go高级实战"}`,
			dbCourse:     dbCourse,
			wantName:     "Go高级实战",
			wantErr:      false,
			wantDbCalls:  0,
			wantCacheSet: false,
		},
		{
			name:         "未命中，查DB并写回缓存",
			cacheData:    "",
			dbCourse:     dbCourse,
			wantName:     "Go高级实战",
			wantErr:      false,
			wantDbCalls:  1,
			wantCacheSet: true,
		},
		{
			name:         "DB报错，不写缓存",
			cacheData:    "",
			dbErr:        errors.New("db down"),
			wantErr:      true,
			wantDbCalls:  1,
			wantCacheSet: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fCourse := &fakeCourse{dbCourse: c.dbCourse, dbErr: c.dbErr}
			fRedis := &fakeRedis{cache: map[int64]string{id: c.cacheData}}
			s := &Service{course: fCourse, rdsCourse: fRedis}
			course, err := s.getCourseInfoCached(ctx, id)

			if (err != nil) != c.wantErr {
				t.Fatalf("err%v, wantErr=%v", err, c.wantErr)
			}
			if err == nil && course.Name != c.wantName {
				t.Errorf("Course, Name=%q, want %q", course.Name, c.wantName)
			}
			if fCourse.called != c.wantDbCalls {
				t.Errorf("DB called=%d want%d", fCourse.called, c.wantDbCalls)
			}
			if c.wantCacheSet {
				if _, ok := fRedis.cache[id]; !ok {
					t.Errorf("cache应被写回，但cache[%d]不存在", id)
				}
			}
		})
	}
}
