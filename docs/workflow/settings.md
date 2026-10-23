---
name: "settings workflow"
description: "settings相关固定处理流程."
---

## 流程

1. data/default/settings.user.json 新增配置条目, data/schema/settings.user.schema.json 同步.
2. frontend/src/static/default/settings.json 同步条目组件信息配置.
3. 按照配置语义在对应的 service 中进行处理.
4. 必要时编写相关的测试.

## 细节

1. 后端 settings 负责配置操作和可改, 为配置唯一来源; 前端 settings 用于控制组件显示内容.
2. 

