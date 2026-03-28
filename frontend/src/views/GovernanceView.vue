<template>
  <div class="governance-page" data-testid="governance-page">
    <el-card class="panel-card">
      <template #header>
        <div class="page-header">
          <div>
            <h3>治理任务中心</h3>
            <p>承接来自治理策略、DQ 告警、结构变更和人工发起的治理任务。</p>
          </div>
          <div class="header-actions">
            <el-button @click="loadTasks" :loading="loading">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
            <el-button type="primary" @click="openCreateDialog">
              <el-icon><Plus /></el-icon>
              新建任务
            </el-button>
          </div>
        </div>
      </template>

      <div class="filters">
        <el-select v-model="filters.status" clearable placeholder="状态" @change="loadTasks">
          <el-option label="待处理" value="open" />
          <el-option label="审核中" value="in_review" />
          <el-option label="阻塞中" value="blocked" />
          <el-option label="已完成" value="done" />
          <el-option label="已取消" value="cancelled" />
        </el-select>

        <el-select v-model="filters.task_type" clearable placeholder="类型" @change="loadTasks">
          <el-option label="结构变更" value="schema_change" />
          <el-option label="DQ 异常" value="dq_issue" />
          <el-option label="术语审核" value="term_review" />
          <el-option label="分类审核" value="classification_review" />
          <el-option label="设计审核" value="design_review" />
          <el-option label="整改执行" value="remediation" />
          <el-option label="手工任务" value="manual" />
        </el-select>

        <el-select v-model="filters.priority" clearable placeholder="优先级" @change="loadTasks">
          <el-option label="低" value="low" />
          <el-option label="中" value="medium" />
          <el-option label="高" value="high" />
          <el-option label="紧急" value="critical" />
        </el-select>

        <el-input
          v-model="filters.resource_id"
          placeholder="按资源 ID 过滤"
          clearable
          @keyup.enter="loadTasks"
          @clear="loadTasks"
        />
      </div>

      <el-table :data="tasks" v-loading="loading" style="width: 100%">
        <el-table-column prop="title" label="任务标题" min-width="220" />
        <el-table-column prop="task_type" label="类型" width="140">
          <template #default="{ row }">
            <el-tag effect="plain">{{ taskTypeLabel(row.task_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="priority" label="优先级" width="110">
          <template #default="{ row }">
            <el-tag :type="priorityTagType(row.priority)">{{ priorityLabel(row.priority) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="statusTagType(row.status)">{{ statusLabel(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="source_system" label="来源" width="120" />
        <el-table-column prop="assignee_id" label="负责人" width="180">
          <template #default="{ row }">
            {{ userLabel(row.assignee_id) }}
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button size="small" data-testid="governance-detail-button" @click="openDetail(row.id)">
              详情
            </el-button>
            <el-button
              v-if="row.status !== 'done'"
              size="small"
              type="success"
              @click="quickUpdateStatus(row, 'done')"
            >
              完成
            </el-button>
            <el-button
              v-else
              size="small"
              type="warning"
              @click="quickUpdateStatus(row, 'open')"
            >
              重开
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="createDialogVisible" title="新建治理任务" width="760px">
      <el-form :model="createForm" label-width="100px">
        <el-form-item label="任务标题">
          <el-input v-model="createForm.title" placeholder="例如：处理 panel_id 术语归一" />
        </el-form-item>
        <el-form-item label="任务描述">
          <el-input v-model="createForm.description" type="textarea" :rows="4" />
        </el-form-item>
        <div class="form-grid">
          <el-form-item label="任务类型">
            <el-select v-model="createForm.task_type">
              <el-option label="结构变更" value="schema_change" />
              <el-option label="DQ 异常" value="dq_issue" />
              <el-option label="术语审核" value="term_review" />
              <el-option label="分类审核" value="classification_review" />
              <el-option label="设计审核" value="design_review" />
              <el-option label="整改执行" value="remediation" />
              <el-option label="手工任务" value="manual" />
            </el-select>
          </el-form-item>
          <el-form-item label="优先级">
            <el-select v-model="createForm.priority">
              <el-option label="低" value="low" />
              <el-option label="中" value="medium" />
              <el-option label="高" value="high" />
              <el-option label="紧急" value="critical" />
            </el-select>
          </el-form-item>
          <el-form-item label="来源系统">
            <el-input v-model="createForm.source_system" placeholder="fuckcmdb / llm-governor" />
          </el-form-item>
          <el-form-item label="负责人">
            <el-select v-model="createForm.assignee_id" clearable filterable placeholder="可选">
              <el-option
                v-for="user in users"
                :key="user.id"
                :label="user.username"
                :value="user.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="资源类型">
            <el-input v-model="createForm.resource_type" placeholder="column / dq_result" />
          </el-form-item>
          <el-form-item label="资源 ID">
            <el-input v-model="createForm.resource_id" placeholder="col_xxx / dqr_xxx" />
          </el-form-item>
          <el-form-item label="截止时间">
            <el-date-picker
              v-model="createForm.due_at"
              type="datetime"
              value-format="YYYY-MM-DDTHH:mm:ssZ"
              placeholder="可选"
            />
          </el-form-item>
          <el-form-item label="显示名称">
            <el-input v-model="createForm.display_name" placeholder="例如：panel_id" />
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="createDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="submitCreateTask">创建</el-button>
      </template>
    </el-dialog>

    <el-drawer
      v-model="detailVisible"
      size="56%"
      :title="selectedDetail?.task.title || '治理任务详情'"
    >
      <template v-if="selectedDetail">
        <div class="detail-section">
          <div class="detail-grid">
            <el-card class="detail-card" shadow="never">
              <template #header>
                <div class="section-title">任务信息</div>
              </template>
              <el-form :model="taskUpdateForm" label-width="90px">
                <el-form-item label="标题">
                  <el-input v-model="taskUpdateForm.title" />
                </el-form-item>
                <el-form-item label="描述">
                  <el-input v-model="taskUpdateForm.description" type="textarea" :rows="4" />
                </el-form-item>
                <div class="form-grid">
                  <el-form-item label="状态">
                    <el-select v-model="taskUpdateForm.status">
                      <el-option label="待处理" value="open" />
                      <el-option label="审核中" value="in_review" />
                      <el-option label="阻塞中" value="blocked" />
                      <el-option label="已完成" value="done" />
                      <el-option label="已取消" value="cancelled" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="优先级">
                    <el-select v-model="taskUpdateForm.priority">
                      <el-option label="低" value="low" />
                      <el-option label="中" value="medium" />
                      <el-option label="高" value="high" />
                      <el-option label="紧急" value="critical" />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="负责人">
                    <el-select v-model="taskUpdateForm.assignee_id" clearable filterable>
                      <el-option
                        v-for="user in users"
                        :key="user.id"
                        :label="user.username"
                        :value="user.id"
                      />
                    </el-select>
                  </el-form-item>
                  <el-form-item label="截止时间">
                    <el-date-picker
                      v-model="taskUpdateForm.due_at"
                      type="datetime"
                      value-format="YYYY-MM-DDTHH:mm:ssZ"
                    />
                  </el-form-item>
                </div>
                <el-button type="primary" :loading="savingTask" @click="saveTaskDetail">
                  保存任务
                </el-button>
              </el-form>
            </el-card>

            <el-card class="detail-card" shadow="never">
              <template #header>
                <div class="section-title">外部资源引用</div>
              </template>
              <el-empty
                v-if="selectedDetail.external_links.length === 0"
                description="暂无外部资源引用"
              />
              <div v-else class="link-list">
                <div
                  v-for="link in selectedDetail.external_links"
                  :key="link.id"
                  class="link-item"
                >
                  <el-link
                    v-if="link.target_url"
                    class="link-main"
                    data-testid="governance-external-link"
                    :href="link.target_url"
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    {{ link.display_name || link.resource_id }}
                  </el-link>
                  <div v-else class="link-main">{{ link.display_name || link.resource_id }}</div>
                  <div class="link-meta">
                    {{ link.source_system }} / {{ link.resource_type }} / {{ link.resource_id }}
                  </div>
                </div>
              </div>
            </el-card>
          </div>
        </div>

        <div class="detail-grid">
          <el-card class="detail-card" shadow="never">
            <template #header>
              <div class="section-title">审核</div>
            </template>
            <div class="inline-form">
              <el-button
                type="primary"
                plain
                data-testid="governance-open-review-dialog"
                @click="openCreateReviewDialog"
              >
                发起审核
              </el-button>
            </div>
            <el-empty v-if="selectedDetail.reviews.length === 0" description="暂无审核记录" />
            <div v-else class="review-list">
              <div v-for="review in selectedDetail.reviews" :key="review.id" class="review-item">
                <div class="review-header">
                  <div>
                    <strong>{{ reviewTypeLabel(review.review_type) }}</strong>
                    <el-tag :type="statusTagType(review.status)" size="small">
                      {{ reviewStatusLabel(review.status) }}
                    </el-tag>
                    <el-tag
                      v-if="review.status === 'approved'"
                      :type="applyStatusTagType(review.apply_status)"
                      size="small"
                    >
                      {{ applyStatusLabel(review.apply_status) }}
                    </el-tag>
                  </div>
                  <span class="muted">审核人：{{ userLabel(review.reviewer_id) }}</span>
                </div>
                <pre class="json-block">{{ prettyJson(review.proposal_payload) }}</pre>
                <pre v-if="review.decision_payload" class="json-block json-block--subtle">{{
                  prettyJson(review.decision_payload)
                }}</pre>
                <pre v-if="review.apply_result" class="json-block json-block--subtle">{{
                  prettyJson(review.apply_result)
                }}</pre>
                <div v-if="review.apply_error" class="timeline-meta">
                  回写错误：{{ review.apply_error }}
                </div>
                <div v-if="canReview(review)" class="review-actions">
                  <el-button size="small" type="success" @click="openReviewDecision(review, 'approved')">
                    通过
                  </el-button>
                  <el-button size="small" type="danger" @click="openReviewDecision(review, 'rejected')">
                    驳回
                  </el-button>
                </div>
                <div v-else-if="canApply(review)" class="review-actions">
                  <el-button
                    size="small"
                    type="primary"
                    data-testid="governance-apply-review-button"
                    :loading="submittingApply"
                    @click="submitReviewApply(review)"
                  >
                    {{ review.apply_status === 'failed' || review.apply_status === 'dead' ? '重试回写' : '立即回写' }}
                  </el-button>
                </div>
              </div>
            </div>
          </el-card>

          <el-card class="detail-card" shadow="never">
            <template #header>
              <div class="section-title">整改证据</div>
            </template>
            <div class="inline-form inline-form--stack">
              <el-select v-model="evidenceForm.evidence_type" placeholder="证据类型">
                <el-option label="备注" value="note" />
                <el-option label="链接" value="link" />
                <el-option label="SQL" value="sql" />
                <el-option label="截图" value="screenshot" />
                <el-option label="文件引用" value="file" />
              </el-select>
              <el-input v-model="evidenceForm.file_id" placeholder="文件 ID，可选" />
              <el-input
                v-model="evidenceForm.content"
                type="textarea"
                :rows="3"
                placeholder="填写证据内容"
              />
              <el-button type="primary" :loading="submittingEvidence" @click="submitEvidence">
                添加证据
              </el-button>
            </div>
            <el-empty v-if="selectedDetail.evidences.length === 0" description="暂无证据" />
            <div v-else class="timeline-list">
              <div v-for="item in selectedDetail.evidences" :key="item.id" class="timeline-item">
                <div class="timeline-header">
                  <strong>{{ evidenceTypeLabel(item.evidence_type) }}</strong>
                  <span class="muted">{{ formatDate(item.created_at) }}</span>
                </div>
                <div class="timeline-content">{{ item.content || '-' }}</div>
                <div v-if="item.file_id" class="timeline-meta">文件 ID：{{ item.file_id }}</div>
              </div>
            </div>
          </el-card>
        </div>

        <el-card class="detail-card detail-card--full" shadow="never">
          <template #header>
            <div class="section-title">评论</div>
          </template>
          <div class="inline-form inline-form--stack">
            <el-input
              v-model="commentForm.content"
              type="textarea"
              :rows="3"
              placeholder="输入任务评论或人工判断"
            />
            <el-button type="primary" :loading="submittingComment" @click="submitComment">
              发表评论
            </el-button>
          </div>
          <el-empty v-if="selectedDetail.comments.length === 0" description="暂无评论" />
          <div v-else class="timeline-list">
            <div v-for="item in selectedDetail.comments" :key="item.id" class="timeline-item">
              <div class="timeline-header">
                <strong>{{ userLabel(item.created_by) }}</strong>
                <span class="muted">{{ formatDate(item.created_at) }}</span>
              </div>
              <div class="timeline-content">{{ item.content }}</div>
            </div>
          </div>
        </el-card>
      </template>
    </el-drawer>

    <el-dialog v-model="reviewDialogVisible" title="发起治理审核" width="680px">
      <el-form :model="reviewForm" label-width="100px">
        <div class="form-grid">
          <el-form-item label="审核类型">
            <el-select v-model="reviewForm.review_type">
              <el-option label="术语绑定" value="term_binding" />
              <el-option label="分类分级" value="classification" />
              <el-option label="DQ 规则" value="dq_rule" />
              <el-option label="设计校验" value="design_validation" />
              <el-option label="整改结果" value="remediation_result" />
              <el-option label="通用审核" value="generic" />
            </el-select>
          </el-form-item>
          <el-form-item label="审核人">
            <el-select v-model="reviewForm.reviewer_id" filterable>
              <el-option
                v-for="user in users"
                :key="user.id"
                :label="user.username"
                :value="user.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="提案来源">
            <el-input v-model="reviewForm.proposal_source" placeholder="llm-governor / manual" />
          </el-form-item>
        </div>
        <el-form-item label="提案模板">
          <div class="template-buttons">
            <el-button
              size="small"
              data-testid="governance-template-term-binding"
              @click="applyReviewTemplate('term_binding')"
            >
              术语绑定
            </el-button>
            <el-button
              size="small"
              data-testid="governance-template-classification"
              @click="applyReviewTemplate('classification')"
            >
              分类分级
            </el-button>
            <el-button
              size="small"
              data-testid="governance-template-dq-rule"
              @click="applyReviewTemplate('dq_rule')"
            >
              DQ 规则
            </el-button>
            <el-button
              size="small"
              data-testid="governance-template-generic"
              @click="applyReviewTemplate('generic')"
            >
              通用模板
            </el-button>
          </div>
        </el-form-item>
        <el-form-item label="提案 JSON">
          <el-input
            v-model="reviewForm.proposal_payload"
            data-testid="governance-review-json"
            type="textarea"
            :rows="8"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reviewDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submittingReview" @click="submitReview">
          提交审核
        </el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="reviewDecisionDialogVisible" :title="reviewDecisionTitle" width="600px">
      <el-form :model="reviewDecisionForm" label-width="90px">
        <el-form-item label="处理说明">
          <el-input
            v-model="reviewDecisionForm.note"
            type="textarea"
            :rows="6"
            placeholder="输入审核结论、原因或附加说明"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="reviewDecisionDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submittingReviewDecision" @click="submitReviewDecision">
          提交
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { governanceAPI, userAPI } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { formatDate } from '@/utils/format'
import type { GovernanceTask, GovernanceTaskDetail, GovernanceReview, User } from '@/types/api'
import { Plus, Refresh } from '@element-plus/icons-vue'

const authStore = useAuthStore()

const loading = ref(false)
const creating = ref(false)
const savingTask = ref(false)
const submittingEvidence = ref(false)
const submittingComment = ref(false)
const submittingReview = ref(false)
const submittingReviewDecision = ref(false)
const submittingApply = ref(false)

const createDialogVisible = ref(false)
const detailVisible = ref(false)
const reviewDialogVisible = ref(false)
const reviewDecisionDialogVisible = ref(false)

const tasks = ref<GovernanceTask[]>([])
const users = ref<User[]>([])
const selectedDetail = ref<GovernanceTaskDetail | null>(null)
const currentReview = ref<GovernanceReview | null>(null)
const reviewDecisionAction = ref<'approved' | 'rejected'>('approved')

const filters = ref({
  status: '',
  task_type: '',
  priority: '',
  resource_id: '',
})

const createForm = ref({
  title: '',
  description: '',
  task_type: 'manual',
  priority: 'medium',
  source_system: '',
  resource_type: '',
  resource_id: '',
  assignee_id: '',
  due_at: '',
  display_name: '',
})

const taskUpdateForm = ref({
  title: '',
  description: '',
  status: 'open',
  priority: 'medium',
  assignee_id: '',
  due_at: '',
})

const evidenceForm = ref({
  evidence_type: 'note',
  content: '',
  file_id: '',
})

const commentForm = ref({
  content: '',
})

const reviewForm = ref({
  review_type: 'generic',
  reviewer_id: '',
  proposal_source: 'manual',
  proposal_payload: '{\n  "summary": ""\n}',
})

const reviewDecisionForm = ref({
  note: '',
})

const reviewDecisionTitle = computed(() =>
  reviewDecisionAction.value === 'approved' ? '审核通过' : '审核驳回',
)

const currentUserId = computed(() => authStore.currentUser?.id || '')

const suggestReviewType = (taskType?: string) => {
  switch (taskType) {
    case 'term_review':
      return 'term_binding'
    case 'classification_review':
      return 'classification'
    case 'dq_issue':
      return 'dq_rule'
    default:
      return 'generic'
  }
}

const buildReviewTemplate = (reviewType: string) => {
  const task = selectedDetail.value?.task
  const firstLink = selectedDetail.value?.external_links[0]
  const displayName = firstLink?.display_name || task?.resource_id || ''

  const templateMap: Record<string, Record<string, unknown>> = {
    term_binding: {
      summary: `确认 ${displayName || '字段'} 的术语绑定建议`,
      recommendation_type: 'term_binding',
      resource_type: task?.resource_type || 'column',
      resource_id: task?.resource_id || '',
      candidate_term: '',
      reason: '',
    },
    classification: {
      summary: `确认 ${displayName || '字段'} 的分类分级`,
      recommendation_type: 'classification',
      resource_type: task?.resource_type || 'column',
      resource_id: task?.resource_id || '',
      classification: '',
      sensitivity_level: '',
      reason: '',
    },
    dq_rule: {
      summary: `确认 ${displayName || task?.resource_id || '资源'} 的 DQ 规则整改方案`,
      recommendation_type: 'dq_rule',
      resource_type: task?.resource_type || '',
      resource_id: task?.resource_id || '',
      dq_rule: '',
      threshold: '',
      remediation_plan: '',
    },
    generic: {
      summary: task?.title || '',
      recommendation_type: 'generic',
      resource_type: task?.resource_type || '',
      resource_id: task?.resource_id || '',
      notes: '',
    },
  }

  return JSON.stringify(templateMap[reviewType] || templateMap.generic, null, 2)
}

const resetCreateForm = () => {
  createForm.value = {
    title: '',
    description: '',
    task_type: 'manual',
    priority: 'medium',
    source_system: '',
    resource_type: '',
    resource_id: '',
    assignee_id: '',
    due_at: '',
    display_name: '',
  }
}

const loadUsers = async () => {
  try {
    const response = await userAPI.list()
    users.value = response.data?.users || []
  } catch {
    users.value = []
  }
}

const loadTasks = async () => {
  loading.value = true
  try {
    const response = await governanceAPI.list({
      status: filters.value.status || undefined,
      task_type: filters.value.task_type || undefined,
      priority: filters.value.priority || undefined,
      resource_id: filters.value.resource_id || undefined,
    })
    tasks.value = response.data?.tasks || []
  } catch (error) {
    console.error(error)
    ElMessage.error('加载治理任务失败')
  } finally {
    loading.value = false
  }
}

const hydrateTaskForm = (task: GovernanceTask) => {
  taskUpdateForm.value = {
    title: task.title,
    description: task.description || '',
    status: task.status,
    priority: task.priority,
    assignee_id: task.assignee_id || '',
    due_at: task.due_at || '',
  }
}

const openDetail = async (taskId: string) => {
  try {
    const response = await governanceAPI.getDetail(taskId)
    selectedDetail.value = response.data || null
    if (selectedDetail.value) {
      hydrateTaskForm(selectedDetail.value.task)
      detailVisible.value = true
    }
  } catch (error) {
    console.error(error)
    ElMessage.error('加载治理任务详情失败')
  }
}

const refreshSelectedDetail = async () => {
  if (!selectedDetail.value) return
  await openDetail(selectedDetail.value.task.id)
}

const openCreateDialog = () => {
  resetCreateForm()
  createDialogVisible.value = true
}

const submitCreateTask = async () => {
  creating.value = true
  try {
    const externalLinks =
      createForm.value.resource_id && createForm.value.resource_type
        ? [
            {
              source_system: createForm.value.source_system || 'manual',
              resource_type: createForm.value.resource_type,
              resource_id: createForm.value.resource_id,
              display_name: createForm.value.display_name,
            },
          ]
        : []

    await governanceAPI.create({
      title: createForm.value.title,
      description: createForm.value.description,
      task_type: createForm.value.task_type,
      priority: createForm.value.priority,
      source_system: createForm.value.source_system,
      resource_type: createForm.value.resource_type,
      resource_id: createForm.value.resource_id,
      assignee_id: createForm.value.assignee_id,
      due_at: createForm.value.due_at || null,
      external_links: externalLinks,
    })
    ElMessage.success('治理任务已创建')
    createDialogVisible.value = false
    resetCreateForm()
    await loadTasks()
  } catch (error) {
    console.error(error)
    ElMessage.error('创建治理任务失败')
  } finally {
    creating.value = false
  }
}

const saveTaskDetail = async () => {
  if (!selectedDetail.value) return
  savingTask.value = true
  try {
    await governanceAPI.update(selectedDetail.value.task.id, {
      title: taskUpdateForm.value.title,
      description: taskUpdateForm.value.description,
      status: taskUpdateForm.value.status,
      priority: taskUpdateForm.value.priority,
      assignee_id: taskUpdateForm.value.assignee_id,
      due_at: taskUpdateForm.value.due_at || null,
    })
    ElMessage.success('治理任务已更新')
    await loadTasks()
    await refreshSelectedDetail()
  } catch (error) {
    console.error(error)
    ElMessage.error('更新治理任务失败')
  } finally {
    savingTask.value = false
  }
}

const quickUpdateStatus = async (task: GovernanceTask, status: string) => {
  try {
    await governanceAPI.update(task.id, {
      title: task.title,
      description: task.description,
      status,
      priority: task.priority,
      assignee_id: task.assignee_id,
      due_at: task.due_at || null,
    })
    ElMessage.success(status === 'done' ? '任务已完成' : '任务已重新打开')
    await loadTasks()
    if (selectedDetail.value?.task.id === task.id) {
      await refreshSelectedDetail()
    }
  } catch (error) {
    console.error(error)
    ElMessage.error('更新任务状态失败')
  }
}

const submitEvidence = async () => {
  if (!selectedDetail.value) return
  submittingEvidence.value = true
  try {
    await governanceAPI.addEvidence(selectedDetail.value.task.id, {
      evidence_type: evidenceForm.value.evidence_type,
      content: evidenceForm.value.content,
      file_id: evidenceForm.value.file_id,
    })
    ElMessage.success('证据已添加')
    evidenceForm.value = {
      evidence_type: 'note',
      content: '',
      file_id: '',
    }
    await refreshSelectedDetail()
  } catch (error) {
    console.error(error)
    ElMessage.error('添加证据失败')
  } finally {
    submittingEvidence.value = false
  }
}

const submitComment = async () => {
  if (!selectedDetail.value) return
  submittingComment.value = true
  try {
    await governanceAPI.addComment(selectedDetail.value.task.id, {
      content: commentForm.value.content,
    })
    ElMessage.success('评论已发布')
    commentForm.value.content = ''
    await refreshSelectedDetail()
  } catch (error) {
    console.error(error)
    ElMessage.error('发布评论失败')
  } finally {
    submittingComment.value = false
  }
}

const openCreateReviewDialog = () => {
  const reviewType = suggestReviewType(selectedDetail.value?.task.task_type)
  reviewForm.value = {
    review_type: reviewType,
    reviewer_id: '',
    proposal_source: 'manual',
    proposal_payload: buildReviewTemplate(reviewType),
  }
  reviewDialogVisible.value = true
}

const applyReviewTemplate = (reviewType: string) => {
  reviewForm.value.review_type = reviewType
  reviewForm.value.proposal_payload = buildReviewTemplate(reviewType)
}

const submitReview = async () => {
  if (!selectedDetail.value) return
  submittingReview.value = true
  try {
    await governanceAPI.createReview({
      task_id: selectedDetail.value.task.id,
      review_type: reviewForm.value.review_type,
      reviewer_id: reviewForm.value.reviewer_id,
      proposal_source: reviewForm.value.proposal_source,
      proposal_payload: reviewForm.value.proposal_payload,
    })
    ElMessage.success('治理审核已创建')
    reviewDialogVisible.value = false
    await loadTasks()
    await refreshSelectedDetail()
  } catch (error) {
    console.error(error)
    ElMessage.error('创建治理审核失败，请确认 JSON 格式正确')
  } finally {
    submittingReview.value = false
  }
}

const canReview = (review: GovernanceReview) =>
  review.status === 'pending' && review.reviewer_id === currentUserId.value

const canApply = (review: GovernanceReview) =>
  review.status === 'approved' &&
  ['term_binding', 'classification', 'dq_rule'].includes(review.review_type) &&
  review.apply_status !== 'succeeded'

const openReviewDecision = (review: GovernanceReview, action: 'approved' | 'rejected') => {
  currentReview.value = review
  reviewDecisionAction.value = action
  reviewDecisionForm.value.note = ''
  reviewDecisionDialogVisible.value = true
}

const submitReviewDecision = async () => {
  if (!currentReview.value) return
  submittingReviewDecision.value = true
  try {
    const decisionPayload = JSON.stringify({
      decision: reviewDecisionAction.value,
      note: reviewDecisionForm.value.note,
    })
    if (reviewDecisionAction.value === 'approved') {
      await governanceAPI.approveReview(currentReview.value.id, { decision_payload: decisionPayload })
    } else {
      await governanceAPI.rejectReview(currentReview.value.id, { decision_payload: decisionPayload })
    }
    ElMessage.success('审核结论已提交')
    reviewDecisionDialogVisible.value = false
    await loadTasks()
    await refreshSelectedDetail()
  } catch (error) {
    console.error(error)
    ElMessage.error('提交审核结论失败')
  } finally {
    submittingReviewDecision.value = false
  }
}

const submitReviewApply = async (review: GovernanceReview) => {
  submittingApply.value = true
  try {
    await governanceAPI.applyReview(review.id)
    ElMessage.success('回写任务已触发')
    await loadTasks()
    await refreshSelectedDetail()
  } catch (error) {
    console.error(error)
    ElMessage.error('触发回写失败')
  } finally {
    submittingApply.value = false
  }
}

const userLabel = (userId?: string) => {
  if (!userId) return '-'
  const user = users.value.find((item) => item.id === userId)
  return user?.username || userId
}

const statusLabel = (status: string) => {
  const mapping: Record<string, string> = {
    open: '待处理',
    in_review: '审核中',
    blocked: '阻塞中',
    done: '已完成',
    cancelled: '已取消',
  }
  return mapping[status] || status
}

const reviewStatusLabel = (status: string) => {
  const mapping: Record<string, string> = {
    pending: '待审核',
    approved: '已通过',
    rejected: '已驳回',
    cancelled: '已取消',
  }
  return mapping[status] || status
}

const applyStatusLabel = (status: string) => {
  const mapping: Record<string, string> = {
    not_requested: '未回写',
    pending: '待回写',
    processing: '回写中',
    succeeded: '已回写',
    failed: '回写失败',
    dead: '回写终止',
  }
  return mapping[status] || status
}

const priorityLabel = (priority: string) => {
  const mapping: Record<string, string> = {
    low: '低',
    medium: '中',
    high: '高',
    critical: '紧急',
  }
  return mapping[priority] || priority
}

const taskTypeLabel = (taskType: string) => {
  const mapping: Record<string, string> = {
    schema_change: '结构变更',
    dq_issue: 'DQ 异常',
    term_review: '术语审核',
    classification_review: '分类审核',
    design_review: '设计审核',
    remediation: '整改执行',
    manual: '手工任务',
  }
  return mapping[taskType] || taskType
}

const reviewTypeLabel = (reviewType: string) => {
  const mapping: Record<string, string> = {
    term_binding: '术语绑定',
    classification: '分类分级',
    dq_rule: 'DQ 规则',
    design_validation: '设计校验',
    remediation_result: '整改结果',
    generic: '通用审核',
  }
  return mapping[reviewType] || reviewType
}

const evidenceTypeLabel = (type: string) => {
  const mapping: Record<string, string> = {
    note: '备注',
    file: '文件',
    link: '链接',
    sql: 'SQL',
    screenshot: '截图',
  }
  return mapping[type] || type
}

const statusTagType = (status: string) => {
  const mapping: Record<string, string> = {
    open: 'info',
    in_review: 'warning',
    blocked: 'danger',
    done: 'success',
    cancelled: '',
    pending: 'warning',
    approved: 'success',
    rejected: 'danger',
  }
  return mapping[status] || 'info'
}

const applyStatusTagType = (status: string) => {
  const mapping: Record<string, string> = {
    not_requested: 'info',
    pending: 'warning',
    processing: 'warning',
    succeeded: 'success',
    failed: 'danger',
    dead: 'danger',
  }
  return mapping[status] || 'info'
}

const priorityTagType = (priority: string) => {
  const mapping: Record<string, string> = {
    low: 'info',
    medium: '',
    high: 'warning',
    critical: 'danger',
  }
  return mapping[priority] || ''
}

const prettyJson = (value: string) => {
  try {
    return JSON.stringify(JSON.parse(value), null, 2)
  } catch {
    return value
  }
}

onMounted(async () => {
  await Promise.all([loadUsers(), loadTasks()])
})
</script>

<style scoped lang="scss">
.governance-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.panel-card {
  :deep(.el-card__body) {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }
}

.page-header {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: flex-start;

  h3 {
    margin: 0 0 8px;
    font-size: 20px;
    color: var(--fa-text-primary);
  }

  p {
    margin: 0;
    font-size: 13px;
    color: var(--fa-text-muted);
  }
}

.header-actions,
.filters,
.inline-form,
.template-buttons {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.filters {
  :deep(.el-select),
  :deep(.el-input) {
    width: 180px;
  }
}

.detail-section,
.detail-grid {
  display: grid;
  gap: 16px;
}

.detail-grid {
  grid-template-columns: repeat(2, minmax(0, 1fr));
  margin-bottom: 16px;
}

.detail-card {
  height: fit-content;
}

.detail-card--full {
  margin-top: 16px;
}

.section-title {
  font-weight: 600;
  color: var(--fa-text-primary);
}

.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 12px;
}

.inline-form--stack {
  flex-direction: column;
}

.review-list,
.timeline-list,
.link-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.review-item,
.timeline-item,
.link-item {
  padding: 14px;
  border-radius: var(--fa-radius-md);
  background: rgba(255, 255, 255, 0.28);
  border: var(--fa-border-subtle);
}

.review-header,
.timeline-header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  margin-bottom: 10px;
}

.timeline-content,
.link-main {
  color: var(--fa-text-primary);
}

.timeline-meta,
.link-meta,
.muted {
  color: var(--fa-text-muted);
  font-size: 12px;
}

.json-block {
  margin: 0;
  padding: 12px;
  border-radius: var(--fa-radius-sm);
  background: rgba(15, 23, 42, 0.9);
  color: #e2e8f0;
  font-size: 12px;
  line-height: 1.5;
  overflow: auto;
}

.json-block--subtle {
  margin-top: 10px;
  background: rgba(30, 41, 59, 0.82);
}

.review-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}

@media (max-width: 1024px) {
  .detail-grid,
  .form-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 768px) {
  .page-header {
    flex-direction: column;
  }

  .filters {
    :deep(.el-select),
    :deep(.el-input) {
      width: 100%;
    }
  }
}
</style>
