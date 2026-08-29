package database

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"gorm.io/gorm"
)

// grantRule liga uma capacidade que o papel já tem à permissão que ele ganha.
type grantRule struct {
	fromResource string
	fromAction   string
	toResource   string
	toAction     string
}

// occurrenceGrants é a equivalência exata com a capacidade de hoje. Quem
// administra etapas neste momento é quem tem settings.general:write, e é só
// esse que recebe as permissões de funil — roles:write não entra, porque
// concedê-lo alargaria acesso numa migração que promete não alterar o de
// ninguém.
var occurrenceGrants = []grantRule{
	{models.ResourceChat, models.ActionRead, models.ResourceOccurrences, models.ActionRead},
	{models.ResourceChat, models.ActionWrite, models.ResourceOccurrences, models.ActionWrite},
	// As três de etapa, incluindo read: sem ela a guarda de rota do frontend
	// esconde a tela de configuração até de quem pode editar.
	{models.ResourceSettingsGeneral, models.ActionWrite, models.ResourceOccurrenceStages, models.ActionRead},
	{models.ResourceSettingsGeneral, models.ActionWrite, models.ResourceOccurrenceStages, models.ActionWrite},
	{models.ResourceSettingsGeneral, models.ActionWrite, models.ResourceOccurrenceStages, models.ActionDelete},
}

// occurrencePermissionKeys são as permissões que este backfill distribui. A
// guarda de "já foi semeado" compara contra esta lista em vez do tamanho de
// occurrenceGrants, que tem entradas repetidas por origem.
var occurrencePermissionKeys = []string{
	models.ResourceOccurrences + ":" + models.ActionRead,
	models.ResourceOccurrences + ":" + models.ActionWrite,
	models.ResourceOccurrenceStages + ":" + models.ActionRead,
	models.ResourceOccurrenceStages + ":" + models.ActionWrite,
	models.ResourceOccurrenceStages + ":" + models.ActionDelete,
}

// BackfillOccurrencePermissions concede as permissões do CRM aos papéis que já
// têm a capacidade equivalente por chat e por configurações gerais.
//
// Existe porque FixSystemRolePermissions pula qualquer papel que já tenha
// permissões, para não desfazer customizações — o que significa que uma
// permissão nova jamais chega a uma instalação existente por aquele caminho.
//
// É PURAMENTE ADITIVO: nunca revoga nada. Essa é a propriedade que torna o
// rollback seguro, porque a versão anterior autoriza por chat:* e ele
// permanece intacto.
//
// Idempotência é pela forma do dado, como BackfillChatbotFlowGraph: uma
// organização que já tenha qualquer papel com qualquer permissão occurrences%
// é pulada inteira, então isto roda uma vez por organização.
func BackfillOccurrencePermissions(db *gorm.DB) error {
	// As linhas de permissão são criadas por SeedPermissionsAndRoles. Se ainda
	// não existem, não há o que ligar e este boot não faz nada.
	var seeded int64
	if err := db.Model(&models.Permission{}).
		Where("resource IN ?", []string{models.ResourceOccurrences, models.ResourceOccurrenceStages}).
		Count(&seeded).Error; err != nil {
		return fmt.Errorf("failed to count occurrence permissions: %w", err)
	}
	if seeded < int64(len(occurrencePermissionKeys)) {
		return nil
	}

	var pending []uuid.UUID
	if err := db.Raw(`
		SELECT o.id
		FROM organizations o
		WHERE NOT EXISTS (
			SELECT 1
			FROM custom_roles r
			JOIN role_permissions rp ON rp.custom_role_id = r.id
			JOIN permissions p ON p.id = rp.permission_id
			WHERE r.organization_id = o.id
			  AND p.resource LIKE 'occurrences%'
		)`).Scan(&pending).Error; err != nil {
		return fmt.Errorf("failed to list organisations pending the occurrence backfill: %w", err)
	}
	if len(pending) == 0 {
		return nil
	}

	for _, g := range occurrenceGrants {
		if err := db.Exec(`
			INSERT INTO role_permissions (custom_role_id, permission_id)
			SELECT r.id, target.id
			FROM custom_roles r
			JOIN role_permissions rp ON rp.custom_role_id = r.id
			JOIN permissions src ON src.id = rp.permission_id
			CROSS JOIN permissions target
			WHERE r.organization_id IN ?
			  AND src.resource = ? AND src.action = ?
			  AND target.resource = ? AND target.action = ?
			  AND NOT EXISTS (
				SELECT 1 FROM role_permissions existing
				WHERE existing.custom_role_id = r.id
				  AND existing.permission_id = target.id
			  )`,
			pending, g.fromResource, g.fromAction, g.toResource, g.toAction,
		).Error; err != nil {
			return fmt.Errorf("failed to grant %s:%s from %s:%s: %w",
				g.toResource, g.toAction, g.fromResource, g.fromAction, err)
		}
	}

	return nil
}
