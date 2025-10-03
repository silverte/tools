# AWS Multi-Account EC2 Instance Collector

여러 AWS 계정의 EC2 인스턴스 정보를 동시에 조회하여 CSV 파일로 저장

## 기능

- 여러 AWS 계정의 EC2 인스턴스를 병렬로 조회
- STS AssumeRole을 사용한 크로스 계정 접근
- 인스턴스 ID, Name 태그, 인스턴스 타입 수집
- 콘솔 테이블 형식 출력
- 타임스탬프가 포함된 CSV 파일 자동 생성

## 사전 요구사항

- Go 1.19 이상
- AWS 자격 증명 설정 (AWS CLI 또는 환경 변수)
- 각 대상 계정에 AssumeRole 권한

## 설치

```bash
go mod init tools
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/credentials/stscreds
go get github.com/aws/aws-sdk-go-v2/service/ec2
go get github.com/aws/aws-sdk-go-v2/service/sts
```

## 사용 방법

### 1. 계정 설정 파일 생성

`accounts.json` 파일을 생성하고 조회할 AWS 계정 정보를 입력:

```json
[
  {
    "account_id": "123456789012",
    "role_arn": "arn:aws:iam::123456789012:role/EC2ReadRole"
  },
  {
    "account_id": "987654321098",
    "role_arn": "arn:aws:iam::987654321098:role/EC2ReadRole"
  }
]
```

### 2. 프로그램 실행

```bash
go run main.go accounts.json
```

### 3. 결과 확인

- 콘솔에 테이블 형식으로 결과 출력
- `ec2_instances_YYYYMMDD_HHMMSS.csv` 파일 생성

## 출력 예시

```
===============================================================================================
Account ID      Instance ID          Tag Name                       Instance Type       
===============================================================================================
123456789012    i-0123456789abcdef0  web-server-01                  t3.medium           
987654321098    i-abcdef0123456789  db-server-prod                 r5.large            
===============================================================================================
총 2개 인스턴스 조회 완료

✓ CSV 파일 저장 완료: ec2_instances_20240115_143025.csv
```

## IAM 권한 설정

대상 계정의 Role에 다음 권한이 필요:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeInstances"
      ],
      "Resource": "*"
    }
  ]
}
```

신뢰 관계 설정:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::SOURCE_ACCOUNT_ID:root"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
```

## 라이선스

MIT
