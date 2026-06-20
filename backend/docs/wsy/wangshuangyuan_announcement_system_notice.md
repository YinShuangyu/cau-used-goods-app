# 公告上线系统消息说明

## 一、改动背景

管理员端公告发布后，用户系统消息页只会展示 `/messages` 接口返回的系统消息。原逻辑中，公告状态改为 `PUBLISHED` 后只更新公告状态并记录管理员日志，没有向用户写入 `messages` 记录，因此普通用户不会收到公告通知。

## 二、改动目标

公告上线后，普通用户可以在前端“系统消息”中看到平台公告通知。

## 三、后端改动

### 1. 公告发布时生成系统消息

修改文件：

```text
backend/internal/admin/service.go
backend/internal/admin/repository.go
```

当公告状态变为 `PUBLISHED` 时，后端会批量写入 `messages` 表，消息类型为：

```text
SYSTEM_NOTICE
```

消息关联对象为：

```text
related_type = NOTICE
related_id = 公告ID
```

### 2. 触发场景

以下两种情况都会触发系统消息生成：

```text
1. 新增公告时直接设置 status = PUBLISHED
2. 已有公告从 DRAFT / OFFLINE 更新为 PUBLISHED
```

### 3. 接收用户范围

只给普通用户发送公告系统消息，筛选条件为：

```text
is_deleted = 0
account_status <> CANCELED
role NOT IN ('ADMIN', 'SUPER_ADMIN')
```

也就是说，管理员和超级管理员不会收到这条面向普通用户的公告通知。

### 4. 防重复发送

同一个公告不会给同一个用户重复生成多条系统消息。

判断依据：

```text
receiver_id + message_type = SYSTEM_NOTICE + related_type = NOTICE + related_id = 公告ID
```

如果已经存在对应消息，则不会重复插入。

## 四、前端展示逻辑

前端无需额外修改。

用户系统消息页原本已经会展示 `SYSTEM_NOTICE` 类型消息：

```text
frontend/hbuilderx-cau-used-goods-uni-v2/pages/messages/system-messages.vue
```

消息列表接口：

```http
GET /messages
```

公告上线后，用户进入消息页即可看到系统通知。

## 五、消息内容

公告系统消息内容格式如下：

```text
标题：平台公告：公告标题
内容：公告内容
```

如果公告内容为空，则使用公告标题作为消息内容。

## 六、测试建议

### 1. 管理员发布公告

管理员端进入公告管理，新增公告后点击发布，或将已有公告状态改为已发布。

对应接口：

```http
PUT /admin/announcements/:id/status
```

请求体：

```json
{
  "status": "PUBLISHED"
}
```

### 2. 普通用户查看系统消息

普通用户登录前端，进入消息页，再进入系统消息。

预期结果：

```text
可以看到一条类型为 SYSTEM_NOTICE 的平台公告消息。
```

### 3. 验证不会重复发送

对同一个公告重复执行发布操作。

预期结果：

```text
同一个普通用户只会收到一条该公告对应的系统消息。
```

## 七、验证情况

已执行后端编译检查：

```bash
go test ./internal/admin
```

结果：

```text
通过
```
