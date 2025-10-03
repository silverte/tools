package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// EC2Instance는 인스턴스 정보를 담는 구조체
type EC2Instance struct {
	AccountID    string
	InstanceID   string
	TagName      string
	InstanceType string
}

// AccountConfig는 계정 설정 파일 구조
type AccountConfig struct {
	AccountID string `json:"account_id"`
	RoleArn   string `json:"role_arn"`
}

// loadAccountsFromFile은 JSON 파일에서 계정 정보를 로드
func loadAccountsFromFile(filename string) (map[string]string, error) {
	// 파일 읽기
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("파일 읽기 실패: %v", err)
	}

	// JSON 파싱
	var accountConfigs []AccountConfig
	if err := json.Unmarshal(data, &accountConfigs); err != nil {
		return nil, fmt.Errorf("JSON 파싱 실패: %v", err)
	}

	// map으로 변환
	accounts := make(map[string]string)
	for _, acc := range accountConfigs {
		accounts[acc.AccountID] = acc.RoleArn
	}

	return accounts, nil
}

// getEC2Instances는 특정 account의 EC2 인스턴스를 조회
func getEC2Instances(ctx context.Context, accountID, roleArn string, results chan<- []EC2Instance, wg *sync.WaitGroup) {
	defer wg.Done()

	// AWS 기본 설정 로드
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Printf("[%s] AWS 설정 로드 실패: %v", accountID, err)
		results <- []EC2Instance{}
		return
	}

	// Role Assume (다른 계정 접근)
	stsClient := sts.NewFromConfig(cfg)
	creds := stscreds.NewAssumeRoleProvider(stsClient, roleArn)
	cfg.Credentials = creds

	// EC2 클라이언트 생성
	client := ec2.NewFromConfig(cfg)

	// EC2 인스턴스 조회
	result, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		log.Printf("[%s] EC2 인스턴스 조회 실패: %v", accountID, err)
		results <- []EC2Instance{}
		return
	}

	// 인스턴스 정보 수집
	var instances []EC2Instance
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			instanceID := *instance.InstanceId
			instanceType := string(instance.InstanceType)

			// Name 태그 찾기
			tagName := "-"
			for _, tag := range instance.Tags {
				if *tag.Key == "Name" {
					tagName = *tag.Value
					break
				}
			}

			instances = append(instances, EC2Instance{
				AccountID:    accountID,
				InstanceID:   instanceID,
				TagName:      tagName,
				InstanceType: instanceType,
			})
		}
	}

	log.Printf("[%s] %d개 인스턴스 조회 완료", accountID, len(instances))
	results <- instances
}

// writeCSV는 결과를 CSV 파일로 저장
func writeCSV(filename string, instances []EC2Instance) error {
	// 파일 생성
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("파일 생성 실패: %v", err)
	}
	defer file.Close()

	// CSV writer 생성
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 헤더 작성
	header := []string{"Account ID", "Instance ID", "Tag Name", "Instance Type"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("헤더 작성 실패: %v", err)
	}

	// 데이터 작성
	for _, instance := range instances {
		record := []string{
			instance.AccountID,
			instance.InstanceID,
			instance.TagName,
			instance.InstanceType,
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("데이터 작성 실패: %v", err)
		}
	}

	return nil
}

func main() {
	// 입력 파일명 확인
	if len(os.Args) < 2 {
		fmt.Println("사용법: go run main.go <accounts.json>")
		fmt.Println("예시: go run main.go accounts.json")
		os.Exit(1)
	}

	inputFile := os.Args[1]

	// 계정 정보 로드
	log.Printf("계정 정보 로드 중: %s", inputFile)
	accounts, err := loadAccountsFromFile(inputFile)
	if err != nil {
		log.Fatalf("계정 정보 로드 실패: %v", err)
	}
	log.Printf("%d개 계정 로드 완료", len(accounts))

	ctx := context.Background()
	var wg sync.WaitGroup
	results := make(chan []EC2Instance, len(accounts))

	// 각 계정에 대해 goroutine으로 동시 조회
	log.Println("EC2 인스턴스 조회 시작...")
	for accountID, roleArn := range accounts {
		wg.Add(1)
		go getEC2Instances(ctx, accountID, roleArn, results, &wg)
	}

	// 모든 goroutine 완료 대기
	wg.Wait()
	close(results)

	// 결과 수집
	var allInstances []EC2Instance
	for instances := range results {
		allInstances = append(allInstances, instances...)
	}

	// 콘솔 출력
	fmt.Println("\n" + strings.Repeat("=", 95))
	fmt.Printf("%-15s %-20s %-30s %-20s\n", "Account ID", "Instance ID", "Tag Name", "Instance Type")
	fmt.Println(strings.Repeat("=", 95))

	if len(allInstances) == 0 {
		fmt.Println("조회된 EC2 인스턴스가 없습니다.")
	} else {
		for _, instance := range allInstances {
			fmt.Printf("%-15s %-20s %-30s %-20s\n",
				instance.AccountID,
				instance.InstanceID,
				instance.TagName,
				instance.InstanceType,
			)
		}
	}

	fmt.Println(strings.Repeat("=", 95))
	fmt.Printf("총 %d개 인스턴스 조회 완료\n", len(allInstances))

	// CSV 파일 저장
	timestamp := time.Now().Format("20060102_150405")
	outputFile := fmt.Sprintf("ec2_instances_%s.csv", timestamp)

	log.Printf("CSV 파일 저장 중: %s", outputFile)
	if err := writeCSV(outputFile, allInstances); err != nil {
		log.Fatalf("CSV 저장 실패: %v", err)
	}

	fmt.Printf("\n✓ CSV 파일 저장 완료: %s\n", outputFile)
}