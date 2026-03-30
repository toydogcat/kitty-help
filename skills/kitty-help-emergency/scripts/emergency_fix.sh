#!/bin/bash

# Kitty Help Emergency Fix Script
PROJECT_DIR="/home/toby/documents/projects/kitty-help"
CONDA_ENV="toby"

cd "$PROJECT_DIR" || { echo "❌ Could not find project directory: $PROJECT_DIR"; exit 1; }

echo "🚑 Starting Emergency Fix for Kitty Help..."
echo "🐍 Using conda environment: $CONDA_ENV"

# Use conda run to execute run.sh
conda run -n "$CONDA_ENV" ./run.sh
