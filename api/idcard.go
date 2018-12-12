package api

import (
	"fmt"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"laoyuegou.pb/godgame/constants"
	"regexp"
	"strconv"
	"time"
)

var (
	IDCARD_RE = regexp.MustCompile("^(\\d{17}([0-9]|X|x))$")
)

// GetGenderAndBirthdayByIDCardNumber 根据身份证获取性别和出生年月
func GetGenderAndBirthdayByIDCardNumber(idNumber string) (int64, int64, error) {
	if len(idNumber) != 18 {
		return constants.GENDER_UNKNOW, 0, fmt.Errorf("invalid idnumber %s", idNumber)
	}
	genderCode, err := strconv.Atoi(idNumber[16:17])
	if err != nil {
		return constants.GENDER_UNKNOW, 0, err
	}
	var gender int64
	if genderCode%2 == 0 {
		gender = constants.GENDER_FEMALE
	} else {
		gender = constants.GENDER_MALE
	}
	t, err := time.Parse("01/02/2006", fmt.Sprintf("%s/%s/%s", idNumber[10:12], idNumber[12:14], idNumber[6:10]))
	if err != nil {
		return constants.GENDER_UNKNOW, 0, err
	}
	birthday := t.Unix()
	return gender, birthday, nil
}

func GenIDCardURL(objectKey, ossAccessID, ossAccessKey string) string {
	client, err := oss.New("oss-cn-hangzhou.aliyuncs.com", ossAccessID, ossAccessKey)
	if err != nil {
		return ""
	}
	bucket, err := client.Bucket("lyg-private-resource")
	if err != nil {
		return ""
	}
	signedURL, err := bucket.SignURL(objectKey, oss.HTTPGet, 870000)
	if err != nil {
		return ""
	}
	return signedURL
}
