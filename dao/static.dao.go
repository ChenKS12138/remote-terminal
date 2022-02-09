package dao

import (
	resource "github.com/ChenKS12138/remote-terminal/resource"
)


type StaticDao struct{}

func NewStaticDao() *StaticDao {
	return &StaticDao{}
}

func (sd *StaticDao) Get(fp string) ([]byte, error) {
	return resource.ResourceFS.ReadFile(fp)
}
