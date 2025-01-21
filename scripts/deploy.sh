#!/bin/bash
set -e

# 色の定義
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 設定値
APP_NAME="face-emotion-analyzer"
ECR_REPOSITORY="face-emotion-analyzer"
AWS_REGION="ap-northeast-1"
ECS_CLUSTER="production"
ECS_SERVICE="face-emotion-analyzer-service"
DOCKER_TAG=$(git rev-parse --short HEAD)

# 引数の解析
ENVIRONMENT="staging"
while [ "$#" -gt 0 ]; do
    case "$1" in
        -e|--environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -t|--tag)
            DOCKER_TAG="$2"
            shift 2
            ;;
        *)
            echo "不明なオプション: $1"
            exit 1
            ;;
    esac
done

# 環境変数の検証
required_envs=(
    "AWS_ACCESS_KEY_ID"
    "AWS_SECRET_ACCESS_KEY"
    "ECR_REGISTRY"
)

for env in "${required_envs[@]}"; do
    if [ -z "${!env}" ]; then
        echo -e "${RED}エラー: ${env} が設定されていません${NC}"
        exit 1
    fi
done

# デプロイメント関数
deploy() {
    echo -e "${YELLOW}デプロイを開始します: ${ENVIRONMENT}${NC}"

    # AWSにログイン
    echo "ECRにログインしています..."
    aws ecr get-login-password --region ${AWS_REGION} | docker login --username AWS --password-stdin ${ECR_REGISTRY}

    # イメージのビルドとプッシュ
    echo "Dockerイメージをビルドしています..."
    docker build -t ${APP_NAME}:${DOCKER_TAG} \
        --build-arg APP_ENV=${ENVIRONMENT} \
        --build-arg BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
        .

    # イメージのタグ付け
    docker tag ${APP_NAME}:${DOCKER_TAG} ${ECR_REGISTRY}/${ECR_REPOSITORY}:${DOCKER_TAG}
    docker tag ${APP_NAME}:${DOCKER_TAG} ${ECR_REGISTRY}/${ECR_REPOSITORY}:latest

    echo "イメージをプッシュしています..."
    docker push ${ECR_REGISTRY}/${ECR_REPOSITORY}:${DOCKER_TAG}
    docker push ${ECR_REGISTRY}/${ECR_REPOSITORY}:latest

    # ECSタスク定義の更新
    echo "タスク定義を更新しています..."
    TASK_DEFINITION=$(aws ecs describe-task-definition \
        --task-definition ${APP_NAME}-${ENVIRONMENT} \
        --region ${AWS_REGION})

    NEW_TASK_DEFINITION=$(echo ${TASK_DEFINITION} | jq --arg IMAGE "${ECR_REGISTRY}/${ECR_REPOSITORY}:${DOCKER_TAG}" \
        '.taskDefinition | .containerDefinitions[0].image = $IMAGE | del(.taskDefinitionArn, .revision, .status, .requiresAttributes, .compatibilities)')

    aws ecs register-task-definition \
        --region ${AWS_REGION} \
        --cli-input-json "${NEW_TASK_DEFINITION}"

    # サービスの更新
    echo "ECSサービスを更新しています..."
    aws ecs update-service \
        --region ${AWS_REGION} \
        --cluster ${ECS_CLUSTER} \
        --service ${ECS_SERVICE} \
        --task-definition ${APP_NAME}-${ENVIRONMENT} \
        --force-new-deployment

    echo -e "${GREEN}デプロイが完了しました${NC}"
}

# デプロイ前の確認
echo -e "${YELLOW}確認:${NC}"
echo "環境: ${ENVIRONMENT}"
echo "タグ: ${DOCKER_TAG}"
echo "リポジトリ: ${ECR_REGISTRY}/${ECR_REPOSITORY}"
echo -n "続行しますか? [y/N] "
read -r response

if [[ ! "$response" =~ ^[Yy]$ ]]; then
    echo "デプロイをキャンセルしました"
    exit 0
fi

# デプロイの実行
deploy

# デプロイ後の健全性チェック
echo "サービスの健全性を確認しています..."
attempt=1
max_attempts=30
until [ $attempt -gt $max_attempts ]; do
    status=$(aws ecs describe-services \
        --region ${AWS_REGION} \
        --cluster ${ECS_CLUSTER} \
        --services ${ECS_SERVICE} \
        --query 'services[0].deployments[0].rolloutState' \
        --output text)

    if [ "$status" == "COMPLETED" ]; then
        echo -e "${GREEN}デプロイメントが正常に完了しました${NC}"
        exit 0
    fi

    if [ "$status" == "FAILED" ]; then
        echo -e "${RED}デプロイメントが失敗しました${NC}"
        exit 1
    fi

    echo "待機中... (${attempt}/${max_attempts})"
    sleep 10
    ((attempt++))
done

echo -e "${RED}タイムアウト: デプロイメントの完了を確認できませんでした${NC}"
exit 1