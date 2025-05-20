#!/bin/bash

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 获取脚本所在目录的绝对路径
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
echo -e "${YELLOW}脚本目录: $SCRIPT_DIR${NC}"
echo -e "${YELLOW}项目目录: $PROJECT_DIR${NC}"

# 默认值
IMAGE_NAME="songguangzhi/m3u8-web-download"
PLATFORM="linux/amd64,linux/arm64"
PUSH_IMAGE=false
DOCKERFILE="$SCRIPT_DIR/Dockerfile"
BUILD_ARGS=""

# 显示帮助信息
show_help() {
    echo -e "${BLUE}构建 M3U8 下载器 Docker 镜像${NC}"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -p, --platform PLATFORM  指定构建平台 (默认: linux/amd64,linux/arm64)"
    echo "  -t, --tag TAG            指定镜像标签 (默认: latest)"
    echo "  --push                   构建后推送镜像到 Docker Hub"
    echo "  -h, --help               显示帮助信息"
    echo ""
    echo "示例:"
    echo "  $0 --platform linux/arm64              # 仅构建 Linux ARM64 平台镜像"
    echo "  $0 --platform \"linux/amd64,linux/arm64\" # 构建多平台镜像"
    echo "  $0 --push                              # 构建默认平台镜像并推送"
    echo "  $0 --tag v1.0.0                        # 构建指定标签的镜像"
}

# 解析命令行参数
TAG="latest"
while [[ $# -gt 0 ]]; do
    key="$1"
    case $key in
        -p|--platform)
            PLATFORM="$2"
            shift
            shift
            ;;
        -t|--tag)
            TAG="$2"
            shift
            shift
            ;;
        --push)
            PUSH_IMAGE=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo -e "${RED}错误: 未知选项 $1${NC}"
            show_help
            exit 1
            ;;
    esac
done

# 检查 Docker 是否已安装
if ! command -v docker &> /dev/null; then
    echo -e "${RED}错误: Docker 未安装或不在 PATH 中${NC}"
    exit 1
fi

# 检查 Docker buildx 是否可用
if ! docker buildx version &> /dev/null; then
    echo -e "${RED}错误: Docker buildx 插件未安装或不可用${NC}"
    echo -e "${YELLOW}请参考 https://docs.docker.com/buildx/working-with-buildx/ 安装 buildx${NC}"
    exit 1
fi

# 检查 Dockerfile 是否存在
if [ ! -f "$DOCKERFILE" ]; then
    echo -e "${RED}错误: Dockerfile 不存在: $DOCKERFILE${NC}"
    exit 1
fi

echo -e "${GREEN}使用 Dockerfile: $DOCKERFILE${NC}"

# 检查前端dist目录是否存在
if [ ! -d "$PROJECT_DIR/web/dist" ]; then
    echo -e "${YELLOW}警告: 前端dist目录不存在，可能导致构建失败${NC}"
    echo -e "${GREEN}检查项目目录结构...${NC}"
    ls -la "$PROJECT_DIR/web"
fi

# 创建或使用 buildx 构建器
echo -e "${YELLOW}设置 buildx 构建器...${NC}"
BUILDER_NAME="m3u8-builder"

# 检查构建器是否存在
if ! docker buildx inspect "$BUILDER_NAME" &> /dev/null; then
    echo -e "${YELLOW}创建新的 buildx 构建器: $BUILDER_NAME${NC}"
    docker buildx create --name "$BUILDER_NAME" --use
else
    echo -e "${YELLOW}使用现有 buildx 构建器: $BUILDER_NAME${NC}"
    docker buildx use "$BUILDER_NAME"
fi

# 构建命令
echo -e "${BLUE}====================================${NC}"
echo -e "${GREEN}开始构建多平台镜像${NC}"
echo -e "${BLUE}====================================${NC}"

# 切换到项目根目录
cd "$PROJECT_DIR"
echo -e "${GREEN}当前工作目录: $(pwd)${NC}"

# 构建处理
BUILD_STATUS=0

if [ "$PUSH_IMAGE" = true ]; then
    # 推送模式：直接构建并推送多平台镜像
    echo -e "${GREEN}构建并推送多平台镜像 ($PLATFORM)${NC}"
    docker buildx build --platform "$PLATFORM" --push -t "${IMAGE_NAME}:${TAG}" $BUILD_ARGS -f "$DOCKERFILE" .
    BUILD_STATUS=$?
else
    # 本地模式：需要为每个平台单独构建
    # 将平台字符串拆分为数组
    IFS=',' read -ra PLATFORMS <<< "$PLATFORM"
    
    for plat in "${PLATFORMS[@]}"; do
        echo -e "${GREEN}开始构建 $plat 平台镜像${NC}"
        
        # 为特定平台创建标签
        platform_tag=$(echo $plat | tr '/' '-')
        tag_with_platform="${IMAGE_NAME}:${TAG}-${platform_tag}"
        
        # 构建命令
        docker buildx build --platform $plat --load -t "$tag_with_platform" $BUILD_ARGS -f "$DOCKERFILE" .
        current_status=$?
        
        if [ $current_status -ne 0 ]; then
            echo -e "${RED}$plat 平台镜像构建失败${NC}"
            BUILD_STATUS=1
        else
            echo -e "${GREEN}$plat 平台镜像构建成功: $tag_with_platform${NC}"
            
            # 如果是默认平台(linux/amd64)，也标记为无平台后缀的版本
            if [ "$plat" == "linux/amd64" ]; then
                docker tag "$tag_with_platform" "${IMAGE_NAME}:${TAG}"
                echo -e "${GREEN}镜像已标记为默认版本: ${IMAGE_NAME}:${TAG}${NC}"
            fi
        fi
    done
fi

if [ $BUILD_STATUS -eq 0 ]; then
    echo -e "${GREEN}镜像构建成功${NC}"
else
    echo -e "${RED}镜像构建失败 (状态码: $BUILD_STATUS)${NC}"
    exit $BUILD_STATUS
fi

# 显示构建的镜像信息
echo -e "${BLUE}====================================${NC}"
echo -e "${GREEN}构建的镜像列表:${NC}"
docker images | grep "$IMAGE_NAME" | grep "$TAG"

# 显示使用提示
echo -e "${BLUE}====================================${NC}"
echo -e "${GREEN}可以使用以下命令运行容器:${NC}"
echo -e "${YELLOW}docker run -d -p 9100:9100 -v /your/download/path:/data ${IMAGE_NAME}:${TAG}${NC}"
echo -e "${BLUE}====================================${NC}" 