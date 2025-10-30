#!/bin/bash

# Interactive Snapchat Bridge Login Test
# Just run: ./quick_test.sh

echo "🚀 Snapchat Bridge Quick Login Test"
echo "===================================="
echo ""

# Prompt for credentials
read -p "Enter Snapchat username [testthebridge]: " USERNAME
USERNAME=${USERNAME:-testthebridge}

read -sp "Enter Snapchat password: " PASSWORD
echo ""
echo ""

if [ -z "$PASSWORD" ]; then
    echo "❌ Error: Password is required"
    exit 1
fi

echo "Testing login for: $USERNAME"
echo ""

# Run the test using a simple curl command first to check API availability
echo "🌐 Testing Snapchat API availability..."
STATUS=$(curl -s -o /dev/null -w "%{http_code}" https://app.snapchat.com)
echo "   API Status: $STATUS"
echo ""

# Now compile and run our Go test
./test_snapchat_login.sh "$USERNAME" "$PASSWORD"
