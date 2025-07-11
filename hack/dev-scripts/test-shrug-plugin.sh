#!/bin/bash
# Test script for the shrug plugin in the development environment
# Sends webhook events to test the shrug plugin functionality

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

HOOK_PORT=${HOOK_PORT:-8889}
WEBHOOK_SECRET=${WEBHOOK_SECRET:-"development-webhook-secret"}

echo -e "${PURPLE}🧪 Testing Shrug Plugin${NC}"
echo "=========================="

# Helper function to send webhook
send_webhook() {
    local event_type="$1"
    local payload="$2"
    local description="$3"
    
    echo -e "${BLUE}📤 Sending: ${description}${NC}"
    
    # Generate HMAC signature
    local signature=$(echo -n "$payload" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" -binary | base64)
    
    # Send webhook
    local response=$(curl -s -w "%{http_code}" \
        -X POST \
        -H "Content-Type: application/json" \
        -H "X-GitHub-Event: $event_type" \
        -H "X-GitHub-Delivery: test-delivery-$(date +%s)" \
        -H "X-Hub-Signature-256: sha256=$signature" \
        -d "$payload" \
        "http://localhost:$HOOK_PORT/hook")
    
    local http_code="${response: -3}"
    local body="${response%???}"
    
    if [[ "$http_code" == "200" ]]; then
        echo -e "${GREEN}✅ Success (HTTP $http_code)${NC}"
    else
        echo -e "${RED}❌ Failed (HTTP $http_code)${NC}"
        echo "Response: $body"
    fi
    
    echo -e "${YELLOW}⏳ Check the logs to see what Prow would do!${NC}"
    echo
}

# Test 1: Issue comment with /shrug
test_shrug_comment() {
    echo -e "${CYAN}Test 1: Adding shrug with '/shrug' comment${NC}"
    
    local payload=$(cat << 'EOF'
{
  "action": "created",
  "issue": {
    "number": 123,
    "title": "Test issue for shrug plugin",
    "state": "open",
    "user": {
      "login": "testuser",
      "id": 12345
    },
    "labels": []
  },
  "comment": {
    "id": 456,
    "body": "/shrug",
    "user": {
      "login": "contributor",
      "id": 67890
    },
    "html_url": "https://github.com/octocat/Hello-World/issues/123#issuecomment-456"
  },
  "repository": {
    "name": "Hello-World",
    "full_name": "octocat/Hello-World",
    "owner": {
      "login": "octocat",
      "id": 1
    }
  }
}
EOF
    )
    
    send_webhook "issue_comment" "$payload" "Issue comment with '/shrug'"
}

# Test 2: Issue comment with /unshrug
test_unshrug_comment() {
    echo -e "${CYAN}Test 2: Removing shrug with '/unshrug' comment${NC}"
    
    local payload=$(cat << 'EOF'
{
  "action": "created",
  "issue": {
    "number": 123,
    "title": "Test issue for shrug plugin",
    "state": "open",
    "user": {
      "login": "testuser",
      "id": 12345
    },
    "labels": [
      {
        "name": "do-not-merge/hold",
        "color": "e11d21"
      }
    ]
  },
  "comment": {
    "id": 789,
    "body": "/unshrug",
    "user": {
      "login": "contributor",
      "id": 67890
    },
    "html_url": "https://github.com/octocat/Hello-World/issues/123#issuecomment-789"
  },
  "repository": {
    "name": "Hello-World",
    "full_name": "octocat/Hello-World",
    "owner": {
      "login": "octocat",
      "id": 1
    }
  }
}
EOF
    )
    
    send_webhook "issue_comment" "$payload" "Issue comment with '/unshrug'"
}

# Test 3: PR comment with /shrug
test_pr_shrug_comment() {
    echo -e "${CYAN}Test 3: Adding shrug to PR with '/shrug' comment${NC}"
    
    local payload=$(cat << 'EOF'
{
  "action": "created",
  "issue": {
    "number": 456,
    "title": "Test PR for shrug plugin",
    "state": "open",
    "pull_request": {},
    "user": {
      "login": "testuser",
      "id": 12345
    },
    "labels": []
  },
  "comment": {
    "id": 999,
    "body": "This looks good! /shrug",
    "user": {
      "login": "reviewer",
      "id": 11111
    },
    "html_url": "https://github.com/octocat/Hello-World/pull/456#issuecomment-999"
  },
  "repository": {
    "name": "Hello-World",
    "full_name": "octocat/Hello-World",
    "owner": {
      "login": "octocat",
      "id": 1
    }
  }
}
EOF
    )
    
    send_webhook "issue_comment" "$payload" "PR comment with '/shrug'"
}

# Test 4: Invalid comment (should be ignored)
test_invalid_comment() {
    echo -e "${CYAN}Test 4: Invalid comment (should be ignored)${NC}"
    
    local payload=$(cat << 'EOF'
{
  "action": "created",
  "issue": {
    "number": 789,
    "title": "Test issue",
    "state": "open",
    "user": {
      "login": "testuser",
      "id": 12345
    },
    "labels": []
  },
  "comment": {
    "id": 111,
    "body": "This is just a regular comment with the word shrug in it",
    "user": {
      "login": "contributor",
      "id": 67890
    },
    "html_url": "https://github.com/octocat/Hello-World/issues/789#issuecomment-111"
  },
  "repository": {
    "name": "Hello-World",
    "full_name": "octocat/Hello-World",
    "owner": {
      "login": "octocat",
      "id": 1
    }
  }
}
EOF
    )
    
    send_webhook "issue_comment" "$payload" "Invalid comment (should be ignored)"
}

# Check if hook is running
check_hook() {
    echo -e "${BLUE}🔍 Checking if hook is running...${NC}"
    
    if curl -s "http://localhost:$HOOK_PORT/plugin-help" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Hook is running and accessible${NC}"
        return 0
    else
        echo -e "${RED}❌ Hook is not accessible at http://localhost:$HOOK_PORT${NC}"
        echo -e "${YELLOW}💡 Make sure to start the development environment first:${NC}"
        echo -e "   ./hack/dev-scripts/start-dev-environment.sh"
        return 1
    fi
}

# Main execution
main() {
    echo -e "${BLUE}🚀 Starting shrug plugin tests...${NC}"
    echo
    
    # Check if hook is running
    if ! check_hook; then
        exit 1
    fi
    
    echo
    echo -e "${YELLOW}💡 Watch the logs in another terminal:${NC}"
    echo -e "   tail -f /workspace/logs/*.log"
    echo
    echo -e "${YELLOW}📋 Look for these log messages:${NC}"
    echo -e "   🚫 BLOCKED GitHub write operation in development mode"
    echo -e "   🏷️  Prow would ADD LABEL to issue/PR"
    echo -e "   💬 Prow would CREATE COMMENT on issue/PR"
    echo
    
    sleep 3
    
    # Run tests
    test_shrug_comment
    sleep 2
    test_unshrug_comment
    sleep 2
    test_pr_shrug_comment
    sleep 2
    test_invalid_comment
    
    echo -e "${GREEN}🎉 All tests completed!${NC}"
    echo
    echo -e "${CYAN}📊 Check the development proxy status:${NC}"
    echo -e "   curl http://localhost:8888/dev/status | jq"
    echo
    echo -e "${CYAN}📈 Check metrics:${NC}"
    echo -e "   curl http://localhost:8890/metrics | grep ghproxy_dev"
    echo
}

# Show usage if help requested
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
    echo "Usage: $0"
    echo
    echo "This script tests the shrug plugin in the development environment."
    echo "It sends various webhook payloads to test different scenarios:"
    echo
    echo "  1. Issue comment with '/shrug' (should add label)"
    echo "  2. Issue comment with '/unshrug' (should remove label and comment)"
    echo "  3. PR comment with '/shrug' (should add label)"
    echo "  4. Invalid comment (should be ignored)"
    echo
    echo "Make sure the development environment is running first:"
    echo "  ./hack/dev-scripts/start-dev-environment.sh"
    echo
    exit 0
fi

# Run main function
main "$@"